package cloudsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sqladmin/v1"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/modules/google/cloud"
	"github.com/pezops/blackstart/util"
)

func init() {
	blackstart.RegisterModule("google_cloudsql_managed_instance", NewCloudSqlManagedInstance)
}

var _ blackstart.Module = &managedInstance{}
var requiredCloudSqlManagedInstanceParameters = []string{inputInstance}

const checkCloudSqlSuperuserRoleQuery = `
SELECT 1
FROM pg_roles AS r
JOIN pg_auth_members AS m ON r.oid = m.roleid
JOIN pg_roles AS u ON u.oid = m.member
WHERE u.rolname = CURRENT_USER AND r.rolname = 'cloudsqlsuperuser';`

// NewCloudSqlManagedInstance creates a new instance of the Cloud SQL managed instance module.
func NewCloudSqlManagedInstance() blackstart.Module {
	return &managedInstance{}
}

type managedInstance struct {
	target     *connectionConfig
	sqlService *sqladmin.Service
	creds      *google.Credentials
}

func (m *managedInstance) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   "google_cloudsql_managed_instance",
		Name: "Google Cloud SQL Managed database instance",
		Description: util.CleanString(
			`
Manages a Google Cloud SQL instance. When managed, the module will ensure that the current workload 
identity is a member of the '''cloudsqlsuperuser''' role on the instance. The instance is then
usable for further operations.

**Notes**

- This module does not create or delete the Cloud SQL instance, it only manages the IAM user access.
- The module uses a temporary built-in user to perform the role management operations. This user is
  created and deleted as needed.
- When the module is set to not exist, the current workload identity is removed from the 
  '''cloudsqlsuperuser''' role, but the user itself is not deleted.
- In Cloud SQL for PostgreSQL, '''cloudsqlsuperuser''' is not a true PostgreSQL '''superuser''' role. For
  grants on database objects (for example tables), the managing role may still need '''WITH GRANT OPTION'''.
  A simple approach is to grant the Blackstart service account role membership in the owner role
  of the target object. Otherwise, the Blackstart service account will need to be granted the same 
  permission '''WITH GRANT OPTION''' on the target object to be able to manage permissions for other users.
- Cloud SQL for SQL Server does not support IAM authentication for database operations and is not supported by this module.
`,
		),
		Requirements: []string{
			"The Cloud SQL instance must exist.",
			"The [Cloud SQL Admin API](https://docs.cloud.google.com/sql/docs/mysql/admin-api) must be enabled on the project.",
			"The instance must have [IAM authentication](https://docs.cloud.google.com/sql/docs/postgres/iam-authentication#instance-config-iam-auth) enabled with the `cloudsql.iam_authentication` / `cloudsql_iam_authentication` flag set to `on`.",
			"The Blackstart service account must have permission to manage, connect, and login to the database instance. Suggested pre-defined roles are [`roles/cloudsql.admin`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.admin), [`roles/cloudsql.client`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.client), and [`roles/cloudsql.instanceUser`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.instanceUser).",
		},
		Inputs: map[string]blackstart.InputValue{
			inputInstance: {
				Description: "Cloud SQL instance ID to manage.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputProject: {
				Description: "Google Cloud project ID. If not provided, the current project will be used.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
			inputDatabase: {
				Description: "Database name to connect to and return in the managed connection.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "postgres",
			},
			inputUser: {
				Description: "The user to manage. If not provided, the current user will be used.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
			inputConnectionType: {
				Description: "Type of connection to use. Must be one of: `PUBLIC_IP`, or `PRIVATE_IP`.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "PRIVATE_IP",
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputConnection: {
				Description: "Database connection to the managed Cloud SQL instance authenticated as the managing user.",
				Type:        reflect.TypeFor[*sql.DB](),
			},
		},
		Examples: map[string]string{
			"Manage a Cloud SQL instance": `id: manage-instance
module: google_cloudsql_managed_instance
inputs:
  instance: my-cloudsql-instance`,
			"Manage instance and grant table privileges": `operations:
  - id: manage-instance
    module: google_cloudsql_managed_instance
    inputs:
      instance: my-cloudsql-instance
      project: my-gcp-project
      connection_type: PRIVATE_IP

  - id: grant-app-user-orders-select
    module: postgres_grant
    inputs:
      connection:
        fromDependency:
          id: manage-instance
          output: connection
      role: app_user
      permission: SELECT
      scope: TABLE
      schema: public
      resource: orders

  - id: grant-app-user-orders-update
    module: postgres_grant
    inputs:
      connection:
        fromDependency:
          id: manage-instance
          output: connection
      role: app_user
      permission: UPDATE
      scope: TABLE
      schema: public
      resource: orders`,
		},
	}
}

func (m *managedInstance) Validate(op blackstart.Operation) error {
	for _, p := range requiredCloudSqlManagedInstanceParameters {
		if _, ok := op.Inputs[p]; !ok {
			return fmt.Errorf("missing required parameter: %s", p)
		}
	}

	instance := op.Inputs[inputInstance]
	if instance.IsStatic() {
		instanceValue, err := blackstart.InputAs[string](instance, true)
		if err != nil {
			return fmt.Errorf("invalid instance: %w", err)
		}
		if instanceValue == "" {
			return fmt.Errorf("instance cannot be empty")
		}
	}

	if ct, ok := op.Inputs[inputConnectionType]; ok && ct.IsStatic() {
		connectionType, err := blackstart.InputAs[string](ct, false)
		if err != nil {
			return fmt.Errorf("invalid connection_type: %w", err)
		}
		if !slices.Contains([]string{"PUBLIC_IP", "PRIVATE_IP", ""}, strings.ToUpper(connectionType)) {
			return fmt.Errorf(
				"invalid connection_type: %s - must be one of: PUBLIC_IP, PRIVATE_IP",
				connectionType,
			)
		}
	}

	return nil
}

func (m *managedInstance) Check(ctx blackstart.ModuleContext) (bool, error) {
	err := m.setup(ctx)
	if err != nil {
		return false, err
	}

	// Check if the instance exists
	exists, err := m.checkInstanceExists(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check instance existence: %w", err)
	}

	if !exists {
		return false, fmt.Errorf("instance %s does not exist in project %s", m.target.instance, m.target.project)
	}

	iamAuthEnabled, err := m.checkInstanceIamAuthenticationEnabled(ctx)
	if err != nil {
		return false, fmt.Errorf("failed to check instance IAM authentication setting: %w", err)
	}
	if !iamAuthEnabled {
		return false, fmt.Errorf(
			"instance %s in project %s does not have cloudsql.iam_authentication enabled",
			m.target.instance,
			m.target.project,
		)
	}

	db, err := m.getConnection(ctx)
	if err != nil {
		if isManagedInstanceBootstrapConnectionError(err) {
			// A failed IAM login here commonly means the IAM DB user/role binding has not been
			// bootstrapped yet. Treat this as "not in desired state" so Set() can reconcile.
			// For doesNotExist mode, an auth failure indicates the managed user/path is already absent.
			if ctx.DoesNotExist() {
				return true, nil
			}
			return false, nil
		}
		return false, fmt.Errorf("failed to open database connection: %w", err)
	}

	isAdmin, err := checkIfSuperuser(ctx, db)
	if err != nil {
		return false, fmt.Errorf("failed to check if user is a member of cloudsqladmin role: %w", err)
	}

	var res bool
	if ctx.DoesNotExist() {
		res = !isAdmin
	} else {
		res = isAdmin
	}

	if res && !ctx.DoesNotExist() {
		err = ctx.Output(outputConnection, db)
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

func (m *managedInstance) Set(ctx blackstart.ModuleContext) error {
	err := m.setup(ctx)
	if err != nil {
		return err
	}

	// Get the temporary admin database connection for management
	tempDb, tempDbClose, err := m.tempAdminDb(ctx)
	if err != nil {
		if tempDb != nil {
			_ = tempDb.Close()
		}
		return fmt.Errorf("failed to open temporary database connection: %w", err)
	}
	defer func() {
		err = tempDbClose()
	}()

	iamUser, err := postgresIamUser(ctx, m.creds)
	if err != nil {
		return err
	}

	userType := iamUserType(m.creds, iamUser)

	err = validateUser(iamUser)
	if err != nil {
		return err
	}

	mgmtUserOp := blackstart.Operation{
		Inputs: map[string]blackstart.Input{
			inputUserType: blackstart.NewInputFromValue(userType),
			inputInstance: blackstart.NewInputFromValue(m.target.instance),
			inputRegion:   blackstart.NewInputFromValue(m.target.region),
			inputProject:  blackstart.NewInputFromValue(m.target.project),
			inputUser:     blackstart.NewInputFromValue(iamUser),
		},
		Id:     "temp-user",
		Name:   "temp-user",
		Module: "google_cloudsql_user",
	}
	mgmtUserModule := user{}
	mgmtUserMctx := blackstart.OpContext(ctx, &mgmtUserOp)
	mgmtUserExists, _ := mgmtUserModule.Check(mgmtUserMctx)

	// Leave the IAM user in place when disabling management using the `doesNotExist` flag, just
	// revoke the role. The management user being removed is a very special case. It seems
	// reasonable that any new management user would delete the old user, if needed.
	if !mgmtUserExists && !ctx.DoesNotExist() {
		err = mgmtUserModule.Set(mgmtUserMctx)
		if err != nil {
			return err
		}
	}

	if !ctx.DoesNotExist() {
		// grant the current user as a member of the cloudsqlsuperuser role
		_, err = tempDb.ExecContext(ctx, fmt.Sprintf("GRANT cloudsqlsuperuser TO \"%v\" WITH ADMIN OPTION;", iamUser))
		if err != nil {
			return fmt.Errorf("failed to grant cloudsqlsuperuser role: %w", err)
		}
		_, err = tempDb.ExecContext(ctx, fmt.Sprintf("ALTER ROLE \"%v\" WITH INHERIT CREATEROLE CREATEDB;", iamUser))
		if err != nil {
			return fmt.Errorf("failed to update managment role: %w", err)
		}
	} else {
		// revoke the current user as a member of the cloudsqlsuperuser role
		_, err = tempDb.ExecContext(ctx, fmt.Sprintf("REVOKE cloudsqlsuperuser FROM \"%v\";", iamUser))
		if err != nil {
			return fmt.Errorf("failed to revoke cloudsqlsuperuser role: %w", err)
		}
	}

	db, err := m.getConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	err = ctx.Output(outputConnection, db)
	if err != nil {
		return err
	}

	return err
}

// setup initializes the module by reading inputs, creating the target configuration and setting
// up the SQL Admin service with the appropriate credentials.
func (m *managedInstance) setup(ctx blackstart.ModuleContext) error {
	m.target = &connectionConfig{}
	projectInput, err := blackstart.ContextInputAs[string](ctx, inputProject, false)
	if err != nil {
		return err
	}
	m.target.project = projectInput
	if m.target.project == "" {
		var creds *google.Credentials
		m.target.project, creds, err = cloud.CurrentProject(ctx)
		if err != nil {
			return err
		}
		if m.creds != nil {
			m.creds = creds
		}
	}

	if m.creds == nil {
		m.creds, err = cloud.DefaultCredentials(ctx)
		if err != nil {
			return err
		}
	}

	instance, err := blackstart.ContextInputAs[string](ctx, inputInstance, true)
	if err != nil {
		return err
	}
	m.target.instance = instance
	if m.target.instance == "" {
		return fmt.Errorf("instance ID cannot be empty")
	}

	database, err := blackstart.ContextInputAs[string](ctx, inputDatabase, false)
	if err != nil && !errors.Is(err, blackstart.ErrInputDoesNotExist) {
		return err
	}
	if database == "" {
		database = "postgres"
	}
	m.target.database = database

	username, err := blackstart.ContextInputAs[string](ctx, inputUser, false)
	if err != nil && !errors.Is(err, blackstart.ErrInputDoesNotExist) {
		return err
	}
	m.target.user = username
	if m.target.user == "" {
		var u string
		u, err = postgresIamUser(ctx, m.creds)

		if err != nil {
			return fmt.Errorf("failed to find the current user: %w", err)
		}
		m.target.user = u
	}

	m.sqlService, err = sqladmin.NewService(ctx, option.WithUserAgent(blackstart.UserAgent))
	if err != nil {
		return fmt.Errorf("failed to create SQL Admin service: %w", err)
	}

	return nil
}

// getConnection returns an active database connection to the target instance.
func (m *managedInstance) getConnection(ctx blackstart.ModuleContext) (*sql.DB, error) {
	var username string
	dbConnIdentifier, err := m.target.connectionIdentifier(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection identifier: %w", err)
	}

	username, err = postgresIamUser(ctx, m.creds)
	if err != nil {
		return nil, fmt.Errorf("failed to find the current user: %w", err)
	}

	dsn := cloudsqlPostgresIamDsn(dbConnIdentifier, m.target.database, username)

	driver, err := m.getDriver(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	var result int
	err = db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		return nil, fmt.Errorf("failed to run query: %w", err)
	}

	return db, nil
}

// getDriver returns the appropriate driver based on the connection type.
func (m *managedInstance) getDriver(ctx blackstart.ModuleContext) (string, error) {
	driver, err := blackstart.ContextInputAs[string](ctx, inputConnectionType, false)
	if err != nil {
		return "", err
	}
	switch strings.ToUpper(driver) {
	case "PUBLIC_IP":
		return sqlDriverPostgresIam, nil
	case "PRIVATE_IP", "":
		return sqlDriverPostgresIamPrivateIp, nil
	default:
		return "", fmt.Errorf("invalid connection_type: %s", driver)
	}
}

// getBuiltinDriver returns the driver name for the built-in user type. This is used for the
// temporary admin connection.
func (m *managedInstance) getBuiltinDriver(ctx blackstart.ModuleContext) (string, error) {
	driver, err := blackstart.ContextInputAs[string](ctx, inputConnectionType, false)
	if err != nil {
		return "", err
	}
	switch strings.ToUpper(driver) {
	case "PUBLIC_IP":
		return sqlDriverPostgres, nil
	case "PRIVATE_IP", "":
		return sqlDriverPostgresPrivateIp, nil
	default:
		return "", fmt.Errorf("invalid connection_type: %s", driver)
	}
}

// tempAdminDb creates a temporary built-in user to perform the role management operations. This
// user is created and deleted as needed. When performing the role management operations, given
// this is only called when the database is not already managed, we can assume the current user
// does not have the `cloudsqlsuperuser` role, so we need to use a built-in user to perform the
// operations. Google Cloud SQL grants built-in users (non-IAM) the `cloudsqlsuperuser` role,
// but IAM users must be explicitly granted the role.
func (m *managedInstance) tempAdminDb(ctx blackstart.ModuleContext) (*sql.DB, func() error, error) {
	const adminDb = "postgres"
	const adminUser = "blackstart"
	closer := func() error { return nil }

	tempPass := util.RandomPassword(22)
	tempUserOp := blackstart.Operation{
		Inputs: map[string]blackstart.Input{
			inputUserType: blackstart.NewInputFromValue(userBuiltIn),
			inputInstance: blackstart.NewInputFromValue(m.target.instance),
			inputRegion:   blackstart.NewInputFromValue(m.target.region),
			inputProject:  blackstart.NewInputFromValue(m.target.project),
			inputDatabase: blackstart.NewInputFromValue(adminDb),
			inputUser:     blackstart.NewInputFromValue(adminUser),
			inputPassword: blackstart.NewInputFromValue(tempPass),
		},
		Id:     "temp-user",
		Name:   "temp-user",
		Module: "google_cloudsql_user",
	}
	tempUserModule := user{}
	tempUserMctx := blackstart.OpContext(ctx, &tempUserOp)
	err := tempUserModule.Set(tempUserMctx)
	if err != nil {
		return nil, closer, err
	}

	tempInstanceIndentifier, err := tempUserModule.target.connectionIdentifier(ctx)
	if err != nil {
		return nil, closer, fmt.Errorf("failed to get temporary connection identifier: %w", err)
	}
	tempConnDsn := cloudsqlPostgresBuiltInDsn(tempInstanceIndentifier, adminDb, adminUser, tempPass)

	driver, err := m.getBuiltinDriver(ctx)
	if err != nil {
		return nil, closer, fmt.Errorf("failed to get temporary driver: %w", err)
	}

	tempDb, err := sql.Open(driver, tempConnDsn)
	if err != nil {
		return nil, closer, fmt.Errorf("failed to open temporary database connection: %w", err)
	}

	closer = func() error {
		tempUserOp.DoesNotExist = true
		tempUserMctx = blackstart.OpContext(ctx, &tempUserOp)
		closeErr := tempUserModule.Set(tempUserMctx)
		if closeErr != nil {
			return closeErr
		}

		return tempDb.Close()
	}

	return tempDb, closer, nil
}

// checkInstanceExists checks if the specified Cloud SQL instance exists in the given project.
func (m *managedInstance) checkInstanceExists(ctx blackstart.ModuleContext) (bool, error) {
	// List instances for the given project
	instancesListCall := m.sqlService.Instances.List(m.target.project)
	instancesList, err := instancesListCall.Context(ctx).Do()
	if err != nil {
		return false, fmt.Errorf("failed to list instances: %w", err)
	}

	// Check if the specified instanceId exists
	for _, instance := range instancesList.Items {
		if instance.Name == m.target.instance {
			return true, nil
		}
	}

	return false, nil
}

// checkInstanceIamAuthenticationEnabled returns true when the instance has the flag
// 'cloudsql.iam_authentication' enabled.
func (m *managedInstance) checkInstanceIamAuthenticationEnabled(ctx blackstart.ModuleContext) (bool, error) {
	instance, err := m.sqlService.Instances.Get(m.target.project, m.target.instance).Context(ctx).Do()
	if err != nil {
		return false, err
	}
	if instance.Settings == nil {
		return false, nil
	}
	for _, flag := range instance.Settings.DatabaseFlags {
		if flag == nil {
			continue
		}
		if strings.EqualFold(flag.Name, "cloudsql.iam_authentication") {
			return strings.EqualFold(flag.Value, "on"), nil
		}
	}
	return false, nil
}

// checkIfSuperuser checks if the current user is a member of the `cloudsqlsuperuser` role on the
// instance.
func checkIfSuperuser(ctx context.Context, db *sql.DB) (bool, error) {
	var exists int
	err := db.QueryRowContext(ctx, checkCloudSqlSuperuserRoleQuery).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check if user is a member of cloudsqlsuperuser role: %w", err)
	}

	return true, nil
}

// isManagedInstanceBootstrapConnectionError detects connection failures that should be treated as
// a check miss (not a fatal error) so managed-instance reconciliation can bootstrap itself.
func isManagedInstanceBootstrapConnectionError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "sqlstate 28p01")
}

// validateUser checks if the provided user is valid for a PostgreSQL role.
func validateUser(id string) error {
	// check non-empty
	if id == "" {
		return fmt.Errorf("user cannot be empty")
	}

	// check length
	if len(id) > 63 {
		return fmt.Errorf("user cannot be longer than 63 characters")
	}

	// check allowed characters
	for _, c := range id {
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '-' || c == '_' || c == '@' || c == '.' {
			// valid character
			continue
		} else {
			return fmt.Errorf("invalid character in user: %c", c)
		}
	}
	return nil
}
