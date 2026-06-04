package cloudsql

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/pezops/blackstart/util"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sqladmin/v1"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/modules/google/cloud"
)

func init() {
	blackstart.RegisterModule("google_cloudsql_user", NewCloudSqlUser)

	validUserTypes = []string{userCloudIamUser, userCloudIamServiceAccount}
}

var _ blackstart.Module = &user{}
var requiredUserParameters = []string{inputInstance, inputUser, inputUserType}

// ErrMySQLUserCollision indicates an IAM user conflicts with an existing MySQL local username.
var ErrMySQLUserCollision = errors.New("MySQL user local-name collision")

// Explicitly choosing to support only IAM service accounts and IAM users / groups. Built-in users
// are only used temporarily by Blackstart to initially set up managed instances.
var validUserTypes []string

// NewCloudSqlUser creates a new instance of the Cloud SQL user module.
func NewCloudSqlUser() blackstart.Module {
	return &user{}
}

// user manages IAM users and service accounts for a Cloud SQL instances.
type user struct {
	target     *connectionConfig
	sqlService *sqladmin.Service
	// runtime provides injectable Cloud SQL Admin API and database dependencies.
	runtime *cloudSQLRuntime
}

// Info returns metadata describing the Cloud SQL user module.
func (c *user) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   "google_cloudsql_user",
		Name: "Google Cloud SQL user",
		Description: util.CleanString(
			`
Ensures that a Cloud SQL user exists with the specified parameters. In alignment with Blackstart's security best practices, this module only supports managing IAM users and service accounts, and does not support built-in users.

**Notes**

- Cloud SQL for SQL Server does not support IAM authentication for database operations and is not supported by this module. Use Active Directory authentication instead for SQL Server instances.
- Cloud SQL for PostgreSQL stores service-account usernames without the '''.gserviceaccount.com''' suffix, so the database username ends with '''@<project>.iam'''.
- Cloud SQL for MySQL 5.7+ IAM users are supported.
- Cloud SQL for MySQL stores IAM database usernames as the lowercase portion before '''@'''. IAM identities with the same local part cannot coexist on one MySQL instance.
- A built-in MySQL user or different IAM user type with the same local database username is reported as a conflict instead of being replaced.
`,
		),
		Requirements: []string{
			"The Cloud SQL instance must exist.",
			"The IAM user or service account specified must exist.",
			"The [Cloud SQL Admin API](https://docs.cloud.google.com/sql/docs/mysql/admin-api) must be enabled on the project.",
			"The instance must have IAM authentication enabled for [PostgreSQL](https://docs.cloud.google.com/sql/docs/postgres/iam-authentication#instance-config-iam-auth) or [MySQL](https://docs.cloud.google.com/sql/docs/mysql/iam-authentication#configure-iam-db-auth) with the engine-specific authentication flag set to `on`.",
			"The Blackstart service account must have permission to manage the database instance. The suggested pre-defined role is [`roles/cloudsql.admin`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.admin).",
		},
		Inputs: map[string]blackstart.InputValue{
			inputInstance: {
				Description: "Cloud SQL instance ID.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputProject: {
				Description: "Google Cloud project ID. If not provided, the current project will be used.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
			inputRegion: {
				Description: "Google Cloud region for the Cloud SQL instance. If not provided, the region will be inferred from the instance ID.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
			inputUser: {
				Description: "Username for the Cloud SQL user.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputUserType: {
				Description: "Type of the user to create. Must be one of: `CLOUD_IAM_USER`, `CLOUD_IAM_SERVICE_ACCOUNT`.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputUser: {
				Description: "The database username of the Cloud SQL user that was created or managed. For MySQL IAM users, this is the local part before `@`.",
				Type:        reflect.TypeFor[string](),
			},
		},
		Examples: map[string]string{
			"Create a Cloud IAM user": `id: create-iam-user
module: google_cloudsql_user
inputs:
  instance: my-cloudsql-instance
  user: my-iam-user@example.com
  user_type: CLOUD_IAM_USER`,
		},
	}
}

// Validate checks whether an operation contains valid Cloud SQL user inputs.
func (c *user) Validate(op blackstart.Operation) error {
	for _, p := range requiredUserParameters {
		if _, ok := op.Inputs[p]; !ok {
			return fmt.Errorf("missing required parameter: %s", p)
		}
	}

	userTypeInput := op.Inputs[inputUserType]
	if !userTypeInput.IsStatic() {
		return nil
	}
	userTypeValue, err := blackstart.InputAs[string](userTypeInput, true)
	if err != nil {
		return fmt.Errorf("invalid user_type: %w", err)
	}
	userType := strings.ToUpper(userTypeValue)
	if !slices.Contains([]string{userCloudIamUser, userCloudIamServiceAccount}, userType) {
		if userType == userBuiltIn {
			return fmt.Errorf("user cannot be a built-in user for security purposes - use an IAM service account instead")
		}

		return fmt.Errorf(
			"invalid user_type: %s - must be one of: %s", userType,
			strings.Join(validUserTypes, ", "),
		)
	}
	return nil
}

// Check reports whether the target Cloud SQL user is in the requested state.
func (c *user) Check(ctx blackstart.ModuleContext) (bool, error) {
	err := c.setup(ctx)
	if err != nil {
		return false, err
	}

	// List users for the given instance
	usersList, err := c.sqlService.Users.List(c.target.project, c.target.instance).Context(ctx).Do()
	if err != nil {
		return false, fmt.Errorf("failed to list users: %w", err)
	}

	if ctx.Tainted() {
		return false, nil
	}

	u, err := c.user(ctx)
	if err != nil {
		return false, err
	}
	if err = validateMySQLUserCollision(usersList, u.Name, c.target.userType, c.target.engine); err != nil {
		return false, err
	}

	var res bool
	if ctx.DoesNotExist() {
		res = !cloudSqlUserExists(usersList, u.Name, c.target.engine)

	} else {
		res = cloudSqlUserIsCorrect(usersList, u.Name, c.target.userType, c.target.engine)
	}

	if res && !ctx.DoesNotExist() {
		outputName, outputErr := c.databaseUsername(u.Name)
		if outputErr != nil {
			return false, outputErr
		}
		err = ctx.Output(outputUser, outputName)
		if err != nil {
			return false, err
		}
	}
	return res, nil

}

// Set reconciles the target Cloud SQL user to the requested state.
func (c *user) Set(ctx blackstart.ModuleContext) error {
	err := c.setup(ctx)
	if err != nil {
		return err
	}

	if c.target == nil {
		return fmt.Errorf("connectionConfig user was not setup")
	}

	if c.sqlService == nil {
		return fmt.Errorf("sql service was not initialized")
	}

	u, err := c.user(ctx)
	if err != nil {
		return err
	}

	outputName, err := c.databaseUsername(u.Name)
	if err != nil {
		return err
	}
	err = ctx.Output(outputUser, outputName)
	if err != nil {
		return err
	}

	if ctx.DoesNotExist() {
		// If the does not exist flag is set, the Check() has determined the user exists and
		// should be deleted.
		return c.deleteUser(ctx, u)
	}

	if ctx.Tainted() {
		_ = c.deleteUser(ctx, u)
	}
	return c.createUser(ctx, u)

}

// setup initializes the target connectionConfig and sqladmin.Service for the user module.
func (c *user) setup(mctx blackstart.ModuleContext) error {
	var err error

	c.target, err = createTargetConnectionConfig(mctx)
	if err != nil {
		return err
	}

	// Create a new SQL Admin Service
	c.runtime = cloudSQLRuntimeOrDefault(c.runtime)
	c.sqlService, err = c.runtime.newSQLAdminService(mctx)
	if err != nil {
		return fmt.Errorf("failed to create SQL Admin service: %w", err)
	}
	instance, err := c.sqlService.Instances.Get(c.target.project, c.target.instance).Context(mctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get instance %s in project %s: %w", c.target.instance, c.target.project, err)
	}
	c.target.engine = instanceEngine(instance.DatabaseVersion)
	c.target.databaseVersion = instance.DatabaseVersion
	c.target.region = instance.Region
	c.target.identifier = fmt.Sprintf("%s:%s:%s", c.target.project, instance.Region, c.target.instance)
	if c.target.engine == "SQLSERVER" || c.target.engine == "UNKNOWN" {
		return fmt.Errorf("the Cloud SQL engine %q is not supported by google_cloudsql_user", c.target.engine)
	}
	if c.target.engine == "MYSQL" && !mysqlIamUserSupported(c.target.databaseVersion) {
		return fmt.Errorf(
			"google_cloudsql_user supports MySQL 5.7+; instance uses %s",
			c.target.databaseVersion,
		)
	}
	if !instanceIamAuthenticationEnabled(instance) {
		return fmt.Errorf(
			"instance %s in project %s does not have IAM database authentication enabled",
			c.target.instance,
			c.target.project,
		)
	}

	return nil
}

// user creates the sqladmin.User object based on the target connectionConfig.
func (c *user) user(ctx blackstart.ModuleContext) (*sqladmin.User, error) {
	var err error
	if c.target == nil {
		c.target, err = createTargetConnectionConfig(ctx)
		if err != nil {
			return nil, err
		}
	}

	username := c.target.user

	switch c.target.userType {
	case userCloudIamServiceAccount:
		if c.target.engine != "MYSQL" {
			username, err = normalizeCloudSQLServiceAccountUsername(username, c.target.project)
			if err != nil {
				return nil, err
			}
		}
	default:
	}

	// create the user resource
	sqlUser := &sqladmin.User{
		Name: username,
		Type: c.target.userType,
	}

	if c.target.userType == userBuiltIn {
		if c.target.engine == "MYSQL" {
			sqlUser.Host = "%"
		}
		if c.target.password != "" {
			sqlUser.Password = c.target.password
		}
	}

	return sqlUser, nil
}

// databaseUsername returns the engine-specific database username for an IAM identity.
func (c *user) databaseUsername(iamIdentity string) (string, error) {
	if c.target != nil && c.target.engine == "MYSQL" && c.target.userType != userBuiltIn {
		return mysqlIamUser(iamIdentity)
	}
	return iamIdentity, nil
}

// normalizeCloudSQLServiceAccountUsername normalizes service-account identities into CloudSQL
// username format: <service-account-name>@<project>.iam.
func normalizeCloudSQLServiceAccountUsername(username, project string) (string, error) {
	trimmed := strings.TrimSpace(username)
	if trimmed == "" {
		return "", fmt.Errorf("service account username cannot be empty")
	}

	// Full GSA email -> Cloud SQL format.
	if strings.HasSuffix(trimmed, ".iam.gserviceaccount.com") {
		return strings.TrimSuffix(trimmed, ".gserviceaccount.com"), nil
	}

	// Already Cloud SQL service-account format.
	if strings.HasSuffix(trimmed, ".iam") && strings.Contains(trimmed, "@") {
		return trimmed, nil
	}

	// Bare service-account name -> Cloud SQL format.
	if strings.Contains(trimmed, "@") {
		return "", fmt.Errorf("invalid service account username format: %s", trimmed)
	}
	if strings.TrimSpace(project) == "" {
		return "", fmt.Errorf("project cannot be empty for service account username normalization")
	}
	return fmt.Sprintf("%s@%s.iam", trimmed, project), nil
}

// createUser creates the user in Cloud SQL. If the user already exists, it is deleted first.
func (c *user) createUser(ctx context.Context, user *sqladmin.User) error {
	// Check if the user already exists. If we are being called we need to delete the user first
	// (if they exist) because the check has failed.
	userList, err := c.sqlService.Users.List(c.target.project, c.target.instance).Context(ctx).Do()
	if err != nil {
		return err
	}
	if err = validateMySQLUserCollision(userList, user.Name, c.target.userType, c.target.engine); err != nil {
		return err
	}
	if cloudSqlUserExists(userList, c.target.user, c.target.engine) {
		err = c.deleteUser(ctx, user)
		if err != nil {
			return err
		}
	}

	// Insert the user
	insertCall := c.sqlService.Users.Insert(c.target.project, c.target.instance, user)
	result, err := insertCall.Context(ctx).Do()

	if err != nil {
		return err
	}

	if result == nil {
		return fmt.Errorf("user insert result was empty")
	}

	if result.HTTPStatusCode < 200 || result.HTTPStatusCode >= 300 {
		return fmt.Errorf("status code error while inserting user: %d", result.HTTPStatusCode)
	}

	return nil
}

// deleteUser deletes the user from Cloud SQL.
func (c *user) deleteUser(ctx context.Context, user *sqladmin.User) error {
	// Delete the user
	deleteCall := c.sqlService.Users.Delete(c.target.project, c.target.instance)
	deleteCall.Name(user.Name)
	if c.target.engine == "MYSQL" && c.target.userType == userBuiltIn {
		deleteCall.Host(user.Host)
	}
	result, err := deleteCall.Context(ctx).Do()

	if err != nil {
		return err
	}

	if result == nil {
		return fmt.Errorf("user delete result was empty")
	}

	if result.HTTPStatusCode < 200 || result.HTTPStatusCode >= 300 {
		return fmt.Errorf("status code error while deleting user: %d", result.HTTPStatusCode)
	}

	return nil
}

// cloudSqlUserIsCorrect checks if the target user is in the list of users, and if it is the
// correct type.
func cloudSqlUserIsCorrect(
	usersList *sqladmin.UsersListResponse, targetUser string, targetUserType string, engine ...string,
) bool {
	isBuiltIn := targetUserType == userBuiltIn
	if isBuiltIn {
		// The API returns a blank string for built-in users
		targetUserType = ""
	}

	targetName := targetUser
	if len(engine) > 0 && engine[0] == "MYSQL" && !isBuiltIn {
		var err error
		targetName, err = mysqlIamUser(targetUser)
		if err != nil {
			return false
		}
	}
	for _, u := range usersList.Items {
		if u.Name == targetName {
			return u.Type == targetUserType
		}
	}
	return false
}

// cloudSqlUserExists checks if the connectionConfig user is in the list of users.
func cloudSqlUserExists(usersList *sqladmin.UsersListResponse, targetUser string, engine ...string) bool {
	if usersList == nil {
		return false
	}
	targetName := targetUser
	if len(engine) > 0 && engine[0] == "MYSQL" {
		if normalized, err := mysqlIamUser(targetUser); err == nil {
			targetName = normalized
		}
	}
	for _, user := range usersList.Items {
		if user.Name == targetName {
			return true
		}
	}
	return false
}

// validateMySQLUserCollision rejects MySQL users that conflict with the target local username.
func validateMySQLUserCollision(
	usersList *sqladmin.UsersListResponse, targetUser string, targetUserType string, engine string,
) error {
	if engine != "MYSQL" || targetUserType == userBuiltIn || usersList == nil {
		return nil
	}
	targetName, err := mysqlIamUser(targetUser)
	if err != nil {
		return err
	}
	for _, existingUser := range usersList.Items {
		if existingUser.Name == targetName && existingUser.Type != targetUserType {
			existingType := existingUser.Type
			if existingType == "" {
				existingType = userBuiltIn
			}
			return fmt.Errorf(
				"%w: user %q conflicts with existing %s user of the same local name",
				ErrMySQLUserCollision,
				targetName,
				existingType,
			)
		}
	}
	return nil
}

// createTargetConnectionConfig creates the target connectionConfig from the module context inputs.
func createTargetConnectionConfig(mctx blackstart.ModuleContext) (*connectionConfig, error) {
	target := connectionConfig{}

	u, err := blackstart.ContextInputAs[string](mctx, inputUser, true)
	if err != nil {
		return nil, err
	}
	target.user = u
	if target.user == "" {
		return nil, fmt.Errorf("user cannot be empty")
	}

	userType, err := blackstart.ContextInputAs[string](mctx, inputUserType, true)
	if err != nil {
		return nil, err
	}
	target.userType = userType

	// password is not user for IAM or service account users
	if userType == userBuiltIn {
		var password string
		password, err = blackstart.ContextInputAs[string](mctx, inputPassword, true)
		if err != nil {
			return nil, err
		}
		target.password = password
	}

	instance, err := blackstart.ContextInputAs[string](mctx, inputInstance, true)
	if err != nil {
		return nil, err
	}
	target.instance = instance
	if target.instance == "" {
		return nil, fmt.Errorf("instance cannot be empty")
	}

	project, err := blackstart.ContextInputAs[string](mctx, inputProject, false)
	if err == nil {
		target.project = project
	}
	if target.project == "" {
		var creds *google.Credentials
		target.project, creds, err = cloud.CurrentProject(mctx)
		if err != nil {
			return nil, err
		}
		if creds != nil {
			target.creds = creds
		}
	}

	region, err := blackstart.ContextInputAs[string](mctx, inputRegion, false)
	if err == nil {
		target.region = region
	}

	return &target, nil
}
