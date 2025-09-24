package postgres

import (
	"context"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

func TestRole(t *testing.T) {

	ctx := context.Background()

	db, teardownPgInstance := createTestInstance(ctx, t)
	defer teardownPgInstance()

	tests := []struct {
		name        string
		setup       func()
		checkResult bool
		role        roleModule
	}{
		{
			name: "create_missing_role",
			role: roleModule{
				op: &blackstart.Operation{
					Inputs: map[string]blackstart.Input{inputName: blackstart.NewInputFromValue("blackstart0")},
				},
				target: &role{
					Name:    "blackstart0",
					Inherit: true,
				},
				db: db,
			},
		},
		{
			name: "fix_incorrect_role",
			setup: func() {
				_, err := db.Exec("CREATE ROLE blackstart1 WITH NOCREATEDB LOGIN;")
				require.NoError(t, err)
			},
			role: roleModule{
				op: &blackstart.Operation{
					Inputs: map[string]blackstart.Input{inputName: blackstart.NewInputFromValue("blackstart1")},
				},
				target: &role{
					Name:     "blackstart1",
					Inherit:  true,
					CreateDb: true,
				},
				db: db,
			},
		},
		{
			name: "role_already_exists",
			setup: func() {
				_, err := db.Exec("CREATE ROLE blackstart2 WITH REPLICATION LOGIN;")
				require.NoError(t, err)
			},
			checkResult: true,
			role: roleModule{
				op: &blackstart.Operation{
					Inputs: map[string]blackstart.Input{inputName: blackstart.NewInputFromValue("blackstart2")},
				},
				target: &role{
					Name:        "blackstart2",
					Inherit:     true,
					Replication: true,
					Login:       true,
				},
				db: db,
			},
		},
		{
			name: "delete_role",
			setup: func() {
				_, err := db.Exec("CREATE ROLE blackstart3;")
				require.NoError(t, err)
			},
			role: roleModule{
				op: &blackstart.Operation{
					Inputs:       map[string]blackstart.Input{inputName: blackstart.NewInputFromValue("blackstart3")},
					DoesNotExist: true,
				},
				target: &role{
					Name:    "blackstart3",
					Inherit: true,
				},
				db: db,
			},
		},
		{
			name:        "delete_nonexistent_role",
			checkResult: true,
			role: roleModule{
				op: &blackstart.Operation{
					Inputs:       map[string]blackstart.Input{inputName: blackstart.NewInputFromValue("blackstart4")},
					DoesNotExist: true,
				},
				target: &role{
					Name:    "blackstart4",
					Inherit: true,
				},
				db: db,
			},
		},
	}

	// run each test
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				var testErr error

				if tt.setup != nil {
					tt.setup()
				}

				// create inputs and known at runtime
				mctx := blackstart.InputsToContext(ctx, tt.role.op.Inputs)

				testErr = tt.role.Validate(*tt.role.op)
				require.NoError(t, testErr)

				check, testErr := tt.role.Check(mctx)
				require.NoError(t, testErr)
				// If the check result is supposed to be true, we are just testing the check
				// method on the existing state.
				if tt.checkResult {
					require.True(t, check)
					return
				}

				// The check result is supposed to be false, testing the set method and the
				// follow-up check which should then return true.
				require.False(t, check)

				testErr = tt.role.Set(mctx)
				require.NoError(t, testErr)

				check, testErr = tt.role.Check(mctx)
				require.NoError(t, testErr)
				require.True(t, check)
			},
		)
	}

}
