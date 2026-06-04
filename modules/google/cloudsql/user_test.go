package cloudsql

import (
	"context"
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
		t.Run(name, func(t *testing.T) {
			require.Equal(t, tt.exists, cloudSqlUserExists(users, tt.targetUser, "MYSQL"))
			require.Equal(t, tt.correct, cloudSqlUserIsCorrect(users, tt.targetUser, tt.targetType, "MYSQL"))
		})
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
		t.Run(name, func(t *testing.T) {
			err := validateMySQLUserCollision(tt.users, "person@example.com", tt.targetType, tt.engine)
			if tt.wantErr {
				require.Error(t, err)
				require.ErrorIs(t, err, ErrMySQLUserCollision)
				return
			}
			require.NoError(t, err)
		})
	}
}
