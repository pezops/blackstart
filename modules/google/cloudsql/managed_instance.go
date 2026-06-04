package cloudsql

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"reflect"
	"slices"
	"strings"

	gomysql "github.com/go-sql-driver/mysql"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
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
var _ io.Closer = &managedInstance{}
var requiredCloudSqlManagedInstanceParameters = []string{inputInstance}

const checkPostgresCloudSqlSuperuserRoleQuery = `
SELECT 1
FROM pg_roles AS r
JOIN pg_auth_members AS m ON r.oid = m.roleid
JOIN pg_roles AS u ON u.oid = m.member
WHERE u.rolname = CURRENT_USER AND r.rolname = 'cloudsqlsuperuser';`

const checkMySQLCloudSqlSuperuserRoleQuery = `
SELECT EXISTS(
  SELECT 1 FROM information_schema.enabled_roles WHERE ROLE_NAME = 'cloudsqlsuperuser'
);`

// NewCloudSqlManagedInstance creates a new instance of the Cloud SQL managed instance module.
func NewCloudSqlManagedInstance() blackstart.Module {
	return &managedInstance{}
}

type managedInstance struct {
	target             *connectionConfig
	sqlService         *sqladmin.Service
	creds              *google.Credentials
	managedConnections []*sql.DB
}

func (m *managedInstance) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   "google_cloudsql_managed_instance",
		Name: "Google Cloud SQL Managed database instance",
		Description: util.CleanString(
			`
Manages a Google Cloud SQL for PostgreSQL or MySQL instance. When managed, the module ensures the
current workload identity is a member of the '''cloudsqlsuperuser''' role on the instance. The
instance is then usable for further operations.

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
- Cloud SQL for MySQL 5.6 is not supported because [IAM database authentication is not supported for MySQL 5.6](https://docs.cloud.google.com/sql/docs/mysql/iam-authentication#restrictions).
- Cloud SQL for MySQL 5.7 IAM users are supported by '''google_cloudsql_user''', but managed-instance administration requires the [role support available in MySQL 8+](https://docs.cloud.google.com/sql/docs/mysql/users#mysql-8.0-user-privileges).
`,
		),
		Requirements: []string{
			"The Cloud SQL instance must exist.",
			"The [Cloud SQL Admin API](https://docs.cloud.google.com/sql/docs/mysql/admin-api) must be enabled on the project.",
			"The instance must have IAM authentication enabled for [PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/iam-authentication#instance-config-iam-auth) or [MySQL](https://docs.cloud.google.com/sql/docs/mysql/iam-authentication#configure-iam-db-auth) with the engine-specific authentication flag set to `on`.",
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
				Description: "Database name to connect to and return in the managed connection. Defaults to `postgres` for PostgreSQL and no database for MySQL.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
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

	db, err := m.getConnection(ctx)
	if err != nil {
		if isManagedInstanceBootstrapConnectionError(err, m.target.engine) {
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
	keepConnectionOpen := false
	defer func() {
		if !keepConnectionOpen {
			_ = db.Close()
		}
	}()

	isAdmin, err := checkIfSuperuser(ctx, db, m.target.engine)
	if err != nil {
		return false, fmt.Errorf("failed to check if user is a member of cloudsqlsuperuser role: %w", err)
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
		m.trackManagedConnection(db)
		keepConnectionOpen = true
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

	iamUser := m.target.user
	if iamUser == "" {
		return fmt.Errorf("failed to resolve managed IAM user")
	}

	userType := iamUserType(m.creds, iamUser)

	userToValidate := iamUser
	if m.target.engine == "MYSQL" {
		userToValidate, err = mysqlIamUser(iamUser)
		if err != nil {
			return err
		}
	}
	err = validateUser(userToValidate)
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
	mgmtUserExists, checkErr := mgmtUserModule.Check(mgmtUserMctx)
	if checkErr != nil && (!ctx.DoesNotExist() || errors.Is(checkErr, ErrMySQLUserCollision)) {
		return fmt.Errorf("failed to check managed Cloud SQL user: %w", checkErr)
	}

	// Leave the IAM user in place when disabling management using the `doesNotExist` flag, just
	// revoke the role. The management user being removed is a very special case. It seems
	// reasonable that any new management user would delete the old user, if needed.
	if !mgmtUserExists && !ctx.DoesNotExist() {
		err = mgmtUserModule.Set(mgmtUserMctx)
		if err != nil {
			return err
		}
	}

	if err = m.setManagedRole(ctx, tempDb, iamUser); err != nil {
		return err
	}

	db, err := m.getConnection(ctx)
	if err != nil {
		return fmt.Errorf("failed to open database connection: %w", err)
	}
	keepConnectionOpen := false
	defer func() {
		if !keepConnectionOpen {
			_ = db.Close()
		}
	}()
	err = ctx.Output(outputConnection, db)
	if err != nil {
		return err
	}
	m.trackManagedConnection(db)
	keepConnectionOpen = true

	return err
}

func (m *managedInstance) setManagedRole(ctx blackstart.ModuleContext, db *sql.DB, iamIdentity string) error {
	switch m.target.engine {
	case "MYSQL":
		return setManagedRoleMySQL(ctx, db, iamIdentity)
	case "POSTGRES":
		return setManagedRolePostgres(ctx, db, iamIdentity)
	default:
		return fmt.Errorf("unsupported Cloud SQL engine for role management: %s", m.target.engine)
	}
}

func setManagedRolePostgres(ctx blackstart.ModuleContext, db *sql.DB, iamIdentity string) error {
	if !ctx.DoesNotExist() {
		if _, err := db.ExecContext(
			ctx, fmt.Sprintf("GRANT cloudsqlsuperuser TO \"%v\" WITH ADMIN OPTION;", iamIdentity),
		); err != nil {
			return fmt.Errorf("failed to grant cloudsqlsuperuser role: %w", err)
		}
		if _, err := db.ExecContext(
			ctx, fmt.Sprintf("ALTER ROLE \"%v\" WITH INHERIT CREATEROLE CREATEDB;", iamIdentity),
		); err != nil {
			return fmt.Errorf("failed to update management role: %w", err)
		}
		return nil
	}
	if _, err := db.ExecContext(ctx, fmt.Sprintf("REVOKE cloudsqlsuperuser FROM \"%v\";", iamIdentity)); err != nil {
		return fmt.Errorf("failed to revoke cloudsqlsuperuser role: %w", err)
	}
	return nil
}

func setManagedRoleMySQL(ctx blackstart.ModuleContext, db *sql.DB, iamIdentity string) error {
	username, err := mysqlIamUser(iamIdentity)
	if err != nil {
		return err
	}
	account := fmt.Sprintf("`%s`@`%%`", strings.ReplaceAll(username, "`", "``"))
	if ctx.DoesNotExist() {
		if _, err = db.ExecContext(ctx, "REVOKE `cloudsqlsuperuser` FROM "+account); err != nil {
			return fmt.Errorf("failed to revoke MySQL cloudsqlsuperuser role: %w", err)
		}
		return nil
	}
	if _, err = db.ExecContext(ctx, "GRANT `cloudsqlsuperuser` TO "+account+" WITH ADMIN OPTION"); err != nil {
		return fmt.Errorf("failed to grant MySQL cloudsqlsuperuser role: %w", err)
	}
	if _, err = db.ExecContext(ctx, "SET DEFAULT ROLE ALL TO "+account); err != nil {
		return fmt.Errorf("failed to set default MySQL cloudsqlsuperuser role: %w", err)
	}
	return nil
}

// Close releases any managed database connections.
func (m *managedInstance) Close() error {
	var closeErr error
	for _, db := range m.managedConnections {
		if db == nil {
			continue
		}
		if err := db.Close(); err != nil {
			closeErr = errors.Join(closeErr, err)
		}
	}
	m.managedConnections = nil
	return closeErr
}

// trackManagedConnection registers a connection for lifecycle cleanup at the
// end of workflow execution.
func (m *managedInstance) trackManagedConnection(db *sql.DB) {
	if db == nil {
		return
	}
	m.managedConnections = append(m.managedConnections, db)
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
		if m.creds == nil {
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
	m.target.database = database

	m.sqlService, err = sqladmin.NewService(ctx, option.WithUserAgent(blackstart.UserAgent))
	if err != nil {
		return fmt.Errorf("failed to create SQL Admin service: %w", err)
	}
	instanceResource, err := m.getInstance(ctx)
	if err != nil {
		return err
	}
	m.target.region = instanceResource.Region
	m.target.engine = instanceEngine(instanceResource.DatabaseVersion)
	m.target.databaseVersion = instanceResource.DatabaseVersion
	switch m.target.engine {
	case "POSTGRES":
		if m.target.database == "" {
			m.target.database = "postgres"
		}
	case "MYSQL":
		if !mysqlManagedInstanceSupported(m.target.databaseVersion) {
			return fmt.Errorf(
				"google_cloudsql_managed_instance supports MySQL 8+; instance uses %s",
				m.target.databaseVersion,
			)
		}
	default:
		return fmt.Errorf(
			"the Cloud SQL engine %q is not supported by google_cloudsql_managed_instance", m.target.engine,
		)
	}
	if !instanceIamAuthenticationEnabled(instanceResource) {
		return fmt.Errorf(
			"instance %s in project %s does not have IAM database authentication enabled",
			m.target.instance,
			m.target.project,
		)
	}

	username, err := blackstart.ContextInputAs[string](ctx, inputUser, false)
	if err != nil && !errors.Is(err, blackstart.ErrInputDoesNotExist) {
		return err
	}
	m.target.user = username
	if m.target.user == "" {
		var u string
		u, err = cloud.IamUser(ctx, m.creds)
		if err != nil {
			return fmt.Errorf("failed to find the current user: %w", err)
		}
		if m.target.engine == "POSTGRES" {
			u = strings.TrimSuffix(u, ".gserviceaccount.com")
		}
		m.target.user = u
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

	username = m.target.user
	if username == "" {
		return nil, fmt.Errorf("failed to resolve managed IAM user")
	}

	driver, err := m.getDriver(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get driver: %w", err)
	}

	var dsn string
	switch m.target.engine {
	case "POSTGRES":
		dsn = cloudsqlPostgresIamDsn(dbConnIdentifier, m.target.database, username)
	case "MYSQL":
		username, err = mysqlIamUser(username)
		if err != nil {
			return nil, err
		}
		dsn = cloudsqlMySQLDsn(driver, dbConnIdentifier, m.target.database, username, "")
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
		if m.target.engine == "MYSQL" {
			return sqlDriverMySQLIam, nil
		}
		return sqlDriverPostgresIam, nil
	case "PRIVATE_IP", "":
		if m.target.engine == "MYSQL" {
			return sqlDriverMySQLIamPrivateIp, nil
		}
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
		if m.target.engine == "MYSQL" {
			return sqlDriverMySQL, nil
		}
		return sqlDriverPostgres, nil
	case "PRIVATE_IP", "":
		if m.target.engine == "MYSQL" {
			return sqlDriverMySQLPrivateIp, nil
		}
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
	const adminUser = "blackstart"
	closer := func() error { return nil }
	adminDb := "postgres"
	if m.target.engine == "MYSQL" {
		adminDb = "mysql"
	}

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
	driver, err := m.getBuiltinDriver(ctx)
	if err != nil {
		return nil, closer, fmt.Errorf("failed to get temporary driver: %w", err)
	}

	var tempConnDsn string
	if m.target.engine == "MYSQL" {
		tempConnDsn = cloudsqlMySQLDsn(driver, tempInstanceIndentifier, adminDb, adminUser, tempPass)
	} else {
		tempConnDsn = cloudsqlPostgresBuiltInDsn(tempInstanceIndentifier, adminDb, adminUser, tempPass)
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

func (m *managedInstance) getInstance(ctx blackstart.ModuleContext) (*sqladmin.DatabaseInstance, error) {
	instance, err := m.sqlService.Instances.Get(m.target.project, m.target.instance).Context(ctx).Do()
	if err != nil {
		if apiErr, ok := errors.AsType[*googleapi.Error](err); ok && apiErr.Code == 404 {
			return nil, fmt.Errorf("instance %s does not exist in project %s", m.target.instance, m.target.project)
		}
		return nil, fmt.Errorf("failed to get instance %s in project %s: %w", m.target.instance, m.target.project, err)
	}
	return instance, nil
}

// checkIfSuperuser checks if the current user is a member of the `cloudsqlsuperuser` role on the
// instance.
func checkIfSuperuser(ctx context.Context, db *sql.DB, engine ...string) (bool, error) {
	query := checkPostgresCloudSqlSuperuserRoleQuery
	if len(engine) > 0 && engine[0] == "MYSQL" {
		query = checkMySQLCloudSqlSuperuserRoleQuery
	}
	var exists int
	err := db.QueryRowContext(ctx, query).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("failed to check if user is a member of cloudsqlsuperuser role: %w", err)
	}

	return exists != 0, nil
}

// isManagedInstanceBootstrapConnectionError detects connection failures that should be treated as
// a check miss (not a fatal error) so managed-instance reconciliation can bootstrap itself.
func isManagedInstanceBootstrapConnectionError(err error, engine ...string) bool {
	if err == nil {
		return false
	}
	if len(engine) > 0 && engine[0] == "MYSQL" {
		var mysqlErr *gomysql.MySQLError
		return errors.As(err, &mysqlErr) && mysqlErr.Number == 1045
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "sqlstate 28p01")
}

// validateUser checks if the provided user is valid for a supported database role.
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
