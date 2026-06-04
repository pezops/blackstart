package cloudsql

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/api/sqladmin/v1"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

// TestUserBuiltin tests the cloudsqlv1 user module with a built-in users. The use of built-in users
// is not normally allowed, and it only used temporarily by blackstart to initially set up the
// blackstart IAM user service account.
func TestUserBuiltin(t *testing.T) {

	// This is a live test, the cloud config pulls settings from the environment.
	cloudConfig := map[string]string{
		inputDatabase: "postgres",
		inputUser:     "blackstart",
	}

	envRequiredConfig := []string{inputProject, inputInstance}
	envOptionalConfig := []string{inputRegion, inputDatabase, inputUser}

	for _, v := range envRequiredConfig {
		cloudConfig[v] = util.GetTestEnvRequiredVar(t, modulePackage, v)
	}

	for _, v := range envOptionalConfig {
		r := util.GetTestEnvOptionalVar(t, modulePackage, v)
		if r != "" {
			cloudConfig[v] = r
		}
	}

	// generate random password
	password := util.RandomPassword(18)

	op := blackstart.Operation{
		Inputs: map[string]blackstart.Input{
			inputUserType: blackstart.NewInputFromValue(userBuiltIn),
			inputInstance: blackstart.NewInputFromValue(cloudConfig[inputInstance]),
			inputProject:  blackstart.NewInputFromValue(cloudConfig[inputProject]),
			inputRegion:   blackstart.NewInputFromValue(cloudConfig[inputRegion]),
			inputDatabase: blackstart.NewInputFromValue(cloudConfig[inputDatabase]),
			inputUser:     blackstart.NewInputFromValue(cloudConfig[inputUser]),
			inputPassword: blackstart.NewInputFromValue(password),
		},
		Id:           "test",
		Name:         "test",
		Module:       "google_cloudsql_user",
		DoesNotExist: true,
		Tainted:      true,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	u := NewCloudSqlUser()
	mctx := blackstart.OpContext(ctx, &op)

	op.Inputs[inputUserType] = blackstart.NewInputFromValue(userCloudIamUser)
	err := u.Validate(op)
	require.NoError(t, err)

	// Builtin users are not normally allowed
	op.Inputs[inputUserType] = blackstart.NewInputFromValue(userBuiltIn)
	err = u.Validate(op)
	require.Error(t, err)

	t.Log("checking tainted user for create / recreate")
	res, err := u.Check(mctx)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, false, res)

	// First, delete the user if it exists
	_ = u.Set(mctx)
	op.DoesNotExist = false
	mctx = blackstart.OpContext(ctx, &op)

	t.Log("setting user for create / recreate")
	err = u.Set(mctx)
	require.NoError(t, err)

	t.Log("checking tainted user post-recreate")
	res, err = u.Check(mctx)
	require.NoError(t, err)
	require.Equal(t, false, res)

	op.Tainted = false
	mctx = blackstart.OpContext(ctx, &op)
	t.Log("checking user without taint")
	res, err = u.Check(mctx)
	require.NoError(t, err)
	require.Equal(t, true, res)

	op.DoesNotExist = true
	mctx = blackstart.OpContext(ctx, &op)

	err = u.Validate(op)
	require.Error(t, err)

	t.Log("checking user for does not exist")
	res, err = u.Check(mctx)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, false, res)

	t.Log("setting user for does not exist")
	err = u.Set(mctx)
	require.NoError(t, err)

	t.Log("checking user for does not exist")
	res, err = u.Check(mctx)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, true, res)

	op.DoesNotExist = false
	mctx = blackstart.OpContext(ctx, &op)
	t.Log("checking user without does not exist")
	res, err = u.Check(mctx)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, false, res)
}

func TestCloudSqlMySQLUserMatching(t *testing.T) {
	users := &sqladmin.UsersListResponse{
		Items: []*sqladmin.User{
			{Name: "person", Type: userCloudIamUser},
			{Name: "blackstart", Type: ""},
		},
	}

	tests := map[string]struct {
		targetUser string
		targetType string
		exists     bool
		correct    bool
	}{
		"IAM user matches local name": {
			targetUser: "person@example.com",
			targetType: userCloudIamUser,
			exists:     true,
			correct:    true,
		},
		"built-in user matches unchanged name": {
			targetUser: "blackstart",
			targetType: userBuiltIn,
			exists:     true,
			correct:    true,
		},
		"different IAM type is incorrect": {
			targetUser: "person@example.com",
			targetType: userCloudIamServiceAccount,
			exists:     true,
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				require.Equal(t, tt.exists, cloudSqlUserExists(users, tt.targetUser, "MYSQL"))
				require.Equal(t, tt.correct, cloudSqlUserIsCorrect(users, tt.targetUser, tt.targetType, "MYSQL"))
			},
		)
	}
}

func TestMySQLDatabaseUsername(t *testing.T) {
	u := user{
		target: &connectionConfig{
			engine:   "MYSQL",
			userType: userCloudIamServiceAccount,
		},
	}
	got, err := u.databaseUsername("blackstart@project.iam.gserviceaccount.com")
	require.NoError(t, err)
	require.Equal(t, "blackstart", got)
}

func TestValidateMySQLUserCollision(t *testing.T) {
	tests := map[string]struct {
		users      *sqladmin.UsersListResponse
		targetType string
		engine     string
		wantErr    bool
	}{
		"matching IAM user": {
			users: &sqladmin.UsersListResponse{Items: []*sqladmin.User{
				{Name: "person", Type: userCloudIamUser},
			}},
			targetType: userCloudIamUser,
			engine:     "MYSQL",
		},
		"built-in local name collision": {
			users: &sqladmin.UsersListResponse{Items: []*sqladmin.User{
				{Name: "person", Type: ""},
			}},
			targetType: userCloudIamUser,
			engine:     "MYSQL",
			wantErr:    true,
		},
		"different IAM type collision": {
			users: &sqladmin.UsersListResponse{Items: []*sqladmin.User{
				{Name: "person", Type: userCloudIamServiceAccount},
			}},
			targetType: userCloudIamUser,
			engine:     "MYSQL",
			wantErr:    true,
		},
		"postgres ignores local names": {
			users: &sqladmin.UsersListResponse{Items: []*sqladmin.User{
				{Name: "person", Type: ""},
			}},
			targetType: userCloudIamUser,
			engine:     "POSTGRES",
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				err := validateMySQLUserCollision(tt.users, "person@example.com", tt.targetType, tt.engine)
				if tt.wantErr {
					require.Error(t, err)
					require.ErrorIs(t, err, ErrMySQLUserCollision)
					return
				}
				require.NoError(t, err)
			},
		)
	}
}

// TestUserCheckWithFakeAdminAPI verifies user check behavior against the fake Admin API.
func TestUserCheckWithFakeAdminAPI(t *testing.T) {
	tests := map[string]struct {
		version      string
		users        []*sqladmin.User
		userName     string
		userType     string
		doesNotExist bool
		tainted      bool
		want         bool
		wantErr      error
	}{
		"existing postgres IAM user": {
			version:  "POSTGRES_17",
			users:    []*sqladmin.User{{Name: "person@example.com", Type: userCloudIamUser}},
			userName: "person@example.com",
			userType: userCloudIamUser,
			want:     true,
		},
		"missing user": {
			version:  "POSTGRES_17",
			userName: "person@example.com",
			userType: userCloudIamUser,
		},
		"missing user satisfies does not exist": {
			version:      "MYSQL_8_4",
			userName:     "person@example.com",
			userType:     userCloudIamUser,
			doesNotExist: true,
			want:         true,
		},
		"existing user fails does not exist": {
			version:      "MYSQL_8_4",
			users:        []*sqladmin.User{{Name: "person", Type: userCloudIamUser}},
			userName:     "person@example.com",
			userType:     userCloudIamUser,
			doesNotExist: true,
		},
		"tainted user always misses": {
			version:  "POSTGRES_17",
			users:    []*sqladmin.User{{Name: "person@example.com", Type: userCloudIamUser}},
			userName: "person@example.com",
			userType: userCloudIamUser,
			tainted:  true,
		},
		"mysql collision errors": {
			version:  "MYSQL_8_4",
			users:    []*sqladmin.User{{Name: "person", Type: ""}},
			userName: "person@example.com",
			userType: userCloudIamUser,
			wantErr:  ErrMySQLUserCollision,
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, tt.version)
				api.users = cloneUsers(tt.users)
				op := testCloudSQLUserOperation(tt.userName, tt.userType)
				op.DoesNotExist = tt.doesNotExist
				op.Tainted = tt.tainted
				ctx := blackstart.OpContext(context.Background(), &op)
				module := &user{runtime: api.runtime(nil)}

				got, err := module.Check(ctx)
				if tt.wantErr != nil {
					require.ErrorIs(t, err, tt.wantErr)
					return
				}
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			},
		)
	}
}

// TestUserSetWithFakeAdminAPI verifies user creation, replacement, deletion, and collision behavior.
func TestUserSetWithFakeAdminAPI(t *testing.T) {
	tests := map[string]struct {
		version      string
		users        []*sqladmin.User
		userName     string
		userType     string
		doesNotExist bool
		tainted      bool
		wantUsers    []*sqladmin.User
		wantInsert   int
		wantDelete   int
		insertedName string
		wantErr      error
	}{
		"creates postgres service account with normalized name": {
			version:      "POSTGRES_17",
			userName:     "svc@project.iam.gserviceaccount.com",
			userType:     userCloudIamServiceAccount,
			wantUsers:    []*sqladmin.User{{Name: "svc@project.iam", Host: "%", Type: userCloudIamServiceAccount}},
			wantInsert:   1,
			insertedName: "svc@project.iam",
		},
		"creates mysql IAM user using full identity": {
			version:      "MYSQL_8_4",
			userName:     "person@example.com",
			userType:     userCloudIamUser,
			wantUsers:    []*sqladmin.User{{Name: "person", IamEmail: "person@example.com", Host: "%", Type: userCloudIamUser}},
			wantInsert:   1,
			insertedName: "person@example.com",
		},
		"replaces tainted user": {
			version:      "POSTGRES_17",
			users:        []*sqladmin.User{{Name: "person@example.com", Host: "%", Type: userCloudIamUser}},
			userName:     "person@example.com",
			userType:     userCloudIamUser,
			tainted:      true,
			wantUsers:    []*sqladmin.User{{Name: "person@example.com", Host: "%", Type: userCloudIamUser}},
			wantInsert:   1,
			wantDelete:   1,
			insertedName: "person@example.com",
		},
		"deletes existing user": {
			version:      "MYSQL_8_4",
			users:        []*sqladmin.User{{Name: "person@example.com", Host: "%", Type: userCloudIamUser}},
			userName:     "person@example.com",
			userType:     userCloudIamUser,
			doesNotExist: true,
			wantDelete:   1,
		},
		"refuses mysql collision": {
			version:  "MYSQL_8_4",
			users:    []*sqladmin.User{{Name: "person", Type: ""}},
			userName: "person@example.com",
			userType: userCloudIamUser,
			wantUsers: []*sqladmin.User{
				{Name: "person", Type: ""},
			},
			wantErr: ErrMySQLUserCollision,
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, tt.version)
				api.users = cloneUsers(tt.users)
				op := testCloudSQLUserOperation(tt.userName, tt.userType)
				op.DoesNotExist = tt.doesNotExist
				op.Tainted = tt.tainted
				ctx := blackstart.OpContext(context.Background(), &op)
				module := &user{runtime: api.runtime(nil)}

				err := module.Set(ctx)
				if tt.wantErr != nil {
					require.ErrorIs(t, err, tt.wantErr)
				} else {
					require.NoError(t, err)
				}
				require.ElementsMatch(t, tt.wantUsers, api.users)
				require.Equal(t, tt.wantInsert, api.requestCount(http.MethodPost, "/users"))
				require.Equal(t, tt.wantDelete, api.requestCount(http.MethodDelete, "/users"))
				if tt.wantInsert > 0 {
					require.Equal(t, tt.insertedName, api.inserted[len(api.inserted)-1].Name)
				}
			},
		)
	}
}

// TestUserAdminAPIFailures verifies Admin API failures are returned by the user module.
func TestUserAdminAPIFailures(t *testing.T) {
	tests := map[string]struct {
		method     string
		pathSuffix string
		call       func(*user, blackstart.ModuleContext) error
	}{
		"instance get": {
			method:     http.MethodGet,
			pathSuffix: "/instances/instance",
			call: func(module *user, ctx blackstart.ModuleContext) error {
				_, err := module.Check(ctx)
				return err
			},
		},
		"user list": {
			method:     http.MethodGet,
			pathSuffix: "/instances/instance/users",
			call: func(module *user, ctx blackstart.ModuleContext) error {
				_, err := module.Check(ctx)
				return err
			},
		},
		"user insert": {
			method:     http.MethodPost,
			pathSuffix: "/instances/instance/users",
			call:       func(module *user, ctx blackstart.ModuleContext) error { return module.Set(ctx) },
		},
		"user delete": {
			method:     http.MethodDelete,
			pathSuffix: "/instances/instance/users",
			call: func(module *user, ctx blackstart.ModuleContext) error {
				return module.Set(ctx)
			},
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, "POSTGRES_17")
				api.users = []*sqladmin.User{{Name: "person@example.com", Host: "%", Type: userCloudIamUser}}
				api.fail[tt.method+" /v1/projects/project"+tt.pathSuffix] = http.StatusInternalServerError
				op := testCloudSQLUserOperation("person@example.com", userCloudIamUser)
				if name == "user delete" {
					op.DoesNotExist = true
				}
				ctx := blackstart.OpContext(context.Background(), &op)

				err := tt.call(&user{runtime: api.runtime(nil)}, ctx)
				require.Error(t, err)
				require.False(t, errors.Is(err, ErrMySQLUserCollision))
			},
		)
	}
}

// TestUserSetupValidationWithFakeAdminAPI verifies engine and IAM authentication validation.
func TestUserSetupValidationWithFakeAdminAPI(t *testing.T) {
	tests := map[string]struct {
		version    string
		disableIAM bool
	}{
		"mysql 5.6 unsupported": {
			version: "MYSQL_5_6",
		},
		"sql server unsupported": {
			version: "SQLSERVER_2022_STANDARD",
		},
		"IAM authentication disabled": {
			version:    "POSTGRES_17",
			disableIAM: true,
		},
	}
	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, tt.version)
				if tt.disableIAM {
					api.instance.Settings.DatabaseFlags[0].Value = "off"
				}
				op := testCloudSQLUserOperation("person@example.com", userCloudIamUser)
				ctx := blackstart.OpContext(context.Background(), &op)
				_, err := (&user{runtime: api.runtime(nil)}).Check(ctx)
				require.Error(t, err)
			},
		)
	}
}
