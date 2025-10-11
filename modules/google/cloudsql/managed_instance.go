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

// NewCloudSqlManagedInstance creates a new instance of the CloudSQL managed instance module.
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
		Name: "Google CloudSQL Managed database instance",
		Description: util.CleanString(
			`
Manages a Google CloudSQL instance. When managed, the module will ensure that the current workload 
identity is a member of the '''cloudsqlsuperuser''' role on the instance. The instance is then 
usable for further operations.

**Requirements**

- The CloudSQL instance must exist.
- The instance must have IAM authentication enabled.
- The current workload identity must have the '''roles/cloudsql.admin''' role on the instance.

**Notes**

- This module does not create or delete the CloudSQL instance, it only manages the IAM user access.
- The module uses a temporary built-in user to perform the role management operations. This user is
  created and deleted as needed.
- When the module is set to not exist, the current workload identity is removed from the 
  '''cloudsqlsuperuser''' role, but the user itself is not deleted.
`,
		),
		Inputs: map[string]blackstart.InputValue{
			inputInstance: {
				Description: "CloudSQL instance ID to manage.",
				Type:        reflect.TypeOf(""),
				Required:    true,
			},
			inputProject: {
				Description: "Google Cloud project ID. If not provided, the current project will be used.",
				Type:        reflect.TypeOf(""),
				Required:    false,
			},
			inputUser: {
				Description: "The user to manage. If not provided, the current user will be used.",
				Type:        reflect.TypeOf(""),
				Required:    false,
			},
			inputConnectionType: {
				Description: "Type of connection to use. Must be one of: `PUBLIC_IP`, or `PRIVATE_IP`. Defaults to `PRIVATE_IP`.",
				Type:        reflect.TypeOf(""),
				Required:    false,
				Default:     "PRIVATE_IP",
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputConnection: {
				Description: "Database connection to the managed CloudSQL instance authenticated as the managing user.",
				Type:        reflect.TypeOf(&sql.DB{}),
			},
		},
		Examples: map[string]string{
			"Manage a CloudSQL instance": `id: manage-instance
module: google_cloudsql_managed_instance
inputs:
  instance: my-cloudsql-instance`,
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
	if instance.String() == "" {
		return fmt.Errorf("instance cannot be empty")
	}

	ct := op.Inputs[inputConnectionType]
	if !slices.Contains([]string{"PUBLIC_IP", "PRIVATE_IP", ""}, strings.ToUpper(ct.String())) {
		return fmt.Errorf("invalid connection_type: %s - must be one of: PUBLIC_IP, PRIVATE_IP", ct.String())
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

	db, err := m.getConnection(ctx)
	if err != nil {
		if strings.Contains(err.Error(), "Cloud SQL IAM user authentication failed for user") {
			return false, fmt.Errorf(
				"failed to open database connection: ensure the instance has IAM authentication enabled: %w", err,
			)
		}
		return false, fmt.Errorf("failed to open database connection: %w", err)
	}

	//// Connect to the instance and run a query to check if the current user is a member of the
	//// cloudsqladmin role.
	//conn, err := cloudSQLConnection(m.target.instance, m.target.project, "postgres")
	//if err != nil {
	//	return false, fmt.Errorf("unable to connect to CloudSQL instance: %w", err)
	//}

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

	userType := iamUserType(m.creds)

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
	mgmtUserExists, _ := mgmtUserModule.Check(ctx)

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
	var instance blackstart.Input
	var username blackstart.Input
	m.target = &connectionConfig{}
	projectInput, err := ctx.Input(inputProject)
	if err != nil {
		return err
	}
	m.target.project = projectInput.String()
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

	instance, err = ctx.Input(inputInstance)
	if err != nil {
		return err
	}
	m.target.instance = instance.String()
	if m.target.instance == "" {
		return fmt.Errorf("instance ID cannot be empty")
	}

	username, err = ctx.Input(inputUser)
	if err != nil && !errors.Is(err, blackstart.ErrInputDoesNotExist) {
		return err
	}
	if username != nil {
		m.target.user = username.String()
	}
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

	dsn := cloudsqlPostgresIamDsn(dbConnIdentifier, "postgres", username)

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
	driver, err := ctx.Input(inputConnectionType)
	if err != nil {
		return "", err
	}
	switch strings.ToUpper(driver.String()) {
	case "PUBLIC_IP":
		return sqlDriverPostgresIam, nil
	case "PRIVATE_IP", "":
		return sqlDriverPostgresIamPrivateIp, nil
	default:
		return "", fmt.Errorf("invalid connection_type: %s", driver.String())
	}
}

// getBuiltinDriver returns the driver name for the built-in user type. This is used for the
// temporary admin connection.
func (m *managedInstance) getBuiltinDriver(ctx blackstart.ModuleContext) (string, error) {
	driver, err := ctx.Input(inputConnectionType)
	if err != nil {
		return "", err
	}
	switch strings.ToUpper(driver.String()) {
	case "PUBLIC_IP":
		return sqlDriverPostgres, nil
	case "PRIVATE_IP", "":
		return sqlDriverPostgresPrivateIp, nil
	default:
		return "", fmt.Errorf("invalid connection_type: %s", driver.String())
	}
}

// tempAdminDb creates a temporary built-in user to perform the role management operations. This
// user is created and deleted as needed. When performing the role management operations, given
// this is only called when the database is not already managed, we can assume the current user
// does not have the `cloudsqlsuperuser` role, so we need to use a built-in user to perform the
// operations. Google CloudSQL grants built-in users (non-IAM) the `cloudsqlsuperuser` role,
// but IAM users must be explicitly granted the role.
func (m *managedInstance) tempAdminDb(ctx blackstart.ModuleContext) (*sql.DB, func() error, error) {
	closer := func() error { return nil }

	tempPass := util.RandomPassword(22)
	tempUserOp := blackstart.Operation{
		Inputs: map[string]blackstart.Input{
			inputUserType: blackstart.NewInputFromValue(userBuiltIn),
			inputInstance: blackstart.NewInputFromValue(m.target.instance),
			inputRegion:   blackstart.NewInputFromValue(m.target.region),
			inputProject:  blackstart.NewInputFromValue(m.target.project),
			inputDatabase: blackstart.NewInputFromValue("postgres"),
			inputUser:     blackstart.NewInputFromValue("blackstart"),
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
	tempConnDsn := cloudsqlPostgresBuiltInDsn(tempInstanceIndentifier, "postgres", "blackstart", tempPass)

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

// checkInstanceExists checks if the specified CloudSQL instance exists in the given project.
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

// validateUser checks if the provided user is valid for a Postgres role.
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
