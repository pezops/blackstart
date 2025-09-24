package cloudsql

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/modules/google/cloud"
	"google.golang.org/api/option"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/sqladmin/v1"
)

func init() {
	blackstart.RegisterModule("google_cloudsql_user", NewCloudSqlUser)

	validUserTypes = []string{userCloudIamUser, userCloudIamServiceAccount}
}

var _ blackstart.Module = &user{}
var requiredUserParameters = []string{inputInstance, inputUser, inputUserType}

// Explicitly choosing to support only IAM service accounts and IAM users / groups. Built-in users
// are only used temporarily by Blackstart to initially set up managed instances.
var validUserTypes []string

// NewCloudSqlUser creates a new instance of the CloudSQL user module.
func NewCloudSqlUser() blackstart.Module {
	return &user{}
}

// user manages IAM users and service accounts for a CloudSQL instances.
type user struct {
	target     *connectionConfig
	sqlService *sqladmin.Service
}

func (c *user) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:          "google_cloudsql_user",
		Name:        "Google CloudSQL user",
		Description: "Ensures that a CloudSQL user exists with the specified parameters.",
		Inputs: map[string]blackstart.InputValue{
			inputInstance: {
				Description: "CloudSQL instance ID.",
				Type:        reflect.TypeOf(""),
				Required:    true,
			},
			inputProject: {
				Description: "Google Cloud project ID. If not provided, the current project will be used.",
				Type:        reflect.TypeOf(""),
				Required:    false,
			},
			inputRegion: {
				Description: "Google Cloud region for the CloudSQL instance. If not provided, the region will be inferred from the instance ID.",
				Type:        reflect.TypeOf(""),
				Required:    false,
			},
			inputUser: {
				Description: "username for the CloudSQL user.",
				Type:        reflect.TypeOf(""),
				Required:    true,
			},
			inputUserType: {
				Description: "Type of the user to create. Must be one of: `CLOUD_IAM_USER`, `CLOUD_IAM_SERVICE_ACCOUNT`.",
				Type:        reflect.TypeOf(""),
				Required:    true,
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputUser: {
				Description: "The name of the CloudSQL user that was created or managed.",
				Type:        reflect.TypeOf(""),
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

func (c *user) Validate(op blackstart.Operation) error {
	for _, p := range requiredUserParameters {
		if _, ok := op.Inputs[p]; !ok {
			return fmt.Errorf("missing required parameter: %s", p)
		}
	}

	userTypeInput := op.Inputs[inputUserType]
	userType := strings.ToUpper(userTypeInput.String())
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

	var res bool
	if ctx.DoesNotExist() {
		res = !cloudSqlUserExists(usersList, u.Name)

	} else {
		res = cloudSqlUserIsCorrect(usersList, u.Name, c.target.userType)
	}

	if res && !ctx.DoesNotExist() {
		err = ctx.Output(outputUser, u.Name)
		if err != nil {
			return false, err
		}
	}
	return res, nil

}

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

	err = ctx.Output(outputUser, u.Name)
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
	c.sqlService, err = sqladmin.NewService(mctx, option.WithUserAgent(blackstart.UserAgent))
	if err != nil {
		return fmt.Errorf("failed to create SQL Admin service: %w", err)
	}

	return nil
}

// user creates the sqladmin.User object based on the target connectionConfig.
func (c *user) user(ctx blackstart.ModuleContext) (*sqladmin.User, error) {
	var err error
	c.target, err = createTargetConnectionConfig(ctx)
	if err != nil {
		return nil, err
	}

	username := c.target.user

	switch c.target.userType {
	case userCloudIamServiceAccount:
		username += fmt.Sprintf("@%s.iam", c.target.project)
	default:
	}

	// create the user resource
	sqlUser := &sqladmin.User{
		Name: username,
		Host: "%",
		Type: c.target.userType,
	}

	if c.target.password != "" && c.target.userType == userBuiltIn {
		sqlUser.Password = c.target.password
	}

	return sqlUser, nil
}

// createUser creates the user in CloudSQL. If the user already exists, it is deleted first.
func (c *user) createUser(ctx context.Context, user *sqladmin.User) error {
	// Check if the user already exists. If we are being called we need to delete the user first
	// (if they exist) because the check has failed.
	userList, err := c.sqlService.Users.List(c.target.project, c.target.instance).Context(ctx).Do()
	if err != nil {
		return err
	}
	if cloudSqlUserExists(userList, c.target.user) {
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

// deleteUser deletes the user from CloudSQL.
func (c *user) deleteUser(ctx context.Context, user *sqladmin.User) error {
	// Delete the user
	deleteCall := c.sqlService.Users.Delete(c.target.project, c.target.instance)
	deleteCall.Name(user.Name)
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
func cloudSqlUserIsCorrect(usersList *sqladmin.UsersListResponse, targetUser string, targetUserType string) bool {
	if targetUserType == userBuiltIn {
		// The API returns a blank string for built-in users
		targetUserType = ""
	}

	for _, u := range usersList.Items {
		if u.Name == targetUser {
			return u.Type == targetUserType
		}
	}
	return false
}

// cloudSqlUserExists checks if the connectionConfig user is in the list of users.
func cloudSqlUserExists(usersList *sqladmin.UsersListResponse, targetUser string) bool {
	if usersList == nil {
		return false
	}
	for _, user := range usersList.Items {
		if user.Name == targetUser {
			return true
		}
	}
	return false
}

// createTargetConnectionConfig creates the target connectionConfig from the module context inputs.
func createTargetConnectionConfig(mctx blackstart.ModuleContext) (*connectionConfig, error) {
	target := connectionConfig{}

	u, err := mctx.Input(inputUser)
	if err != nil {
		return nil, err
	}
	target.user = u.String()
	if target.user == "" {
		return nil, fmt.Errorf("user cannot be empty")
	}

	userType, err := mctx.Input(inputUserType)
	if err != nil {
		return nil, err
	}
	target.userType = userType.String()

	// password is not user for IAM or service account users
	if userType.String() == userBuiltIn {
		var password blackstart.Input
		password, err = mctx.Input(inputPassword)
		if err != nil {
			return nil, err
		}
		target.password = password.String()
	}

	instance, err := mctx.Input(inputInstance)
	if err != nil {
		return nil, err
	}
	target.instance = instance.String()
	if target.instance == "" {
		return nil, fmt.Errorf("instance cannot be empty")
	}

	project, err := mctx.Input(inputProject)
	if err == nil {
		target.project = project.String()
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

	region, err := mctx.Input(inputRegion)
	if err == nil {
		target.region = region.String()
	}

	return &target, nil
}
