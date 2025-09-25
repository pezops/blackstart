package postgres

import (
	"context"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

func TestGrant(t *testing.T) {
	var err error
	ctx := context.Background()
	db, teardownPgInstance := createTestInstance(ctx, t)
	defer teardownPgInstance()

	tests := []struct {
		name        string
		setup       func()
		checkResult bool
		grant       grantModule
		inputs      map[string]blackstart.Input
	}{
		{
			name: "schema_create_grant",
			setup: func() {
				_, err = db.Exec("CREATE ROLE blackstart_0")
				if err != nil {
					t.Fatalf("failed to create Role: %v", err)
				}
			},
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("blackstart_0"),
				inputPermission: blackstart.NewInputFromValue("CREATE"),
				inputSchema:     blackstart.NewInputFromValue("public"),
				inputResource:   blackstart.NewInputFromValue("public"),
				inputScope:      blackstart.NewInputFromValue("schema"),
				inputConnection: blackstart.NewInputFromValue(db),
			},
			grant: grantModule{},
		},
		{
			name: "instance_pg_monitor_grant",
			setup: func() {
				_, err = db.Exec("CREATE ROLE blackstart_1")
				if err != nil {
					t.Fatalf("failed to create Role: %v", err)
				}
			},
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("blackstart_1"),
				inputPermission: blackstart.NewInputFromValue("pg_monitor"),
				inputSchema:     blackstart.NewInputFromValue(""),
				inputResource:   blackstart.NewInputFromValue(""),
				inputScope:      blackstart.NewInputFromValue("instance"),
				inputConnection: blackstart.NewInputFromValue(db),
			},
			grant: grantModule{},
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

				mctx := blackstart.InputsToContext(ctx, tt.inputs)

				//err = tt.grant.Validate()
				//require.NoError(t, testErr)
				//
				check, testErr := tt.grant.Check(mctx)
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

				testErr = tt.grant.Set(mctx)
				require.NoError(t, testErr)

				check, testErr = tt.grant.Check(mctx)
				require.NoError(t, testErr)
				require.True(t, check)
			},
		)
	}

}
