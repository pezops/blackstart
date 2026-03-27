package postgres

import (
	"context"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

func TestGrantValidate_StaticPermissionValidation(t *testing.T) {
	m := grantModule{}

	tests := []struct {
		name    string
		op      blackstart.Operation
		wantErr string
	}{
		{
			name: "rejects_all_tables_with_non_table_scope",
			op: blackstart.Operation{
				Id:     "grant-validate-all-tables-scope",
				Module: "postgres_grant",
				Inputs: map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
					inputRole:       blackstart.NewInputFromValue("app_user"),
					inputPermission: blackstart.NewInputFromValue("CONNECT"),
					inputScope:      blackstart.NewInputFromValue("database"),
					inputResource:   blackstart.NewInputFromValue("appdb"),
					inputAll:        blackstart.NewInputFromValue(true),
				},
			},
			wantErr: "only supported when scope is TABLE, SEQUENCE, FUNCTION, PROCEDURE, or ROUTINE",
		},
		{
			name: "rejects_all_tables_with_resource",
			op: blackstart.Operation{
				Id:     "grant-validate-all-tables-resource",
				Module: "postgres_grant",
				Inputs: map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
					inputRole:       blackstart.NewInputFromValue("app_user"),
					inputPermission: blackstart.NewInputFromValue("SELECT"),
					inputScope:      blackstart.NewInputFromValue("table"),
					inputSchema:     blackstart.NewInputFromValue("public"),
					inputResource:   blackstart.NewInputFromValue("orders"),
					inputAll:        blackstart.NewInputFromValue(true),
				},
			},
			wantErr: "must be empty when all is true",
		},
		{
			name: "accepts_sequence_permissions",
			op: blackstart.Operation{
				Id:     "grant-validate-sequence-permission",
				Module: "postgres_grant",
				Inputs: map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
					inputRole:       blackstart.NewInputFromValue("app_user"),
					inputPermission: blackstart.NewInputFromValue("USAGE"),
					inputScope:      blackstart.NewInputFromValue("sequence"),
					inputSchema:     blackstart.NewInputFromValue("public"),
					inputResource:   blackstart.NewInputFromValue("orders_id_seq"),
				},
			},
		},
		{
			name: "rejects_invalid_sequence_permission",
			op: blackstart.Operation{
				Id:     "grant-validate-sequence-invalid-permission",
				Module: "postgres_grant",
				Inputs: map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
					inputRole:       blackstart.NewInputFromValue("app_user"),
					inputPermission: blackstart.NewInputFromValue("DELETE"),
					inputScope:      blackstart.NewInputFromValue("sequence"),
					inputSchema:     blackstart.NewInputFromValue("public"),
					inputResource:   blackstart.NewInputFromValue("orders_id_seq"),
				},
			},
			wantErr: "invalid permission",
		},
		{
			name: "accepts_function_execute_permission",
			op: blackstart.Operation{
				Id:     "grant-validate-function-permission",
				Module: "postgres_grant",
				Inputs: map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
					inputRole:       blackstart.NewInputFromValue("app_user"),
					inputPermission: blackstart.NewInputFromValue("EXECUTE"),
					inputScope:      blackstart.NewInputFromValue("function"),
					inputSchema:     blackstart.NewInputFromValue("public"),
					inputResource:   blackstart.NewInputFromValue("do_work(integer)"),
				},
			},
		},
		{
			name: "rejects_invalid_function_permission",
			op: blackstart.Operation{
				Id:     "grant-validate-function-invalid-permission",
				Module: "postgres_grant",
				Inputs: map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
					inputRole:       blackstart.NewInputFromValue("app_user"),
					inputPermission: blackstart.NewInputFromValue("USAGE"),
					inputScope:      blackstart.NewInputFromValue("function"),
					inputSchema:     blackstart.NewInputFromValue("public"),
					inputResource:   blackstart.NewInputFromValue("do_work(integer)"),
				},
			},
			wantErr: "invalid permission",
		},
		{
			name: "rejects_comma_delimited_permission",
			op: blackstart.Operation{
				Id:     "grant-validate-comma",
				Module: "postgres_grant",
				Inputs: map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
					inputRole:       blackstart.NewInputFromValue("app_user"),
					inputPermission: blackstart.NewInputFromValue("SELECT,UPDATE"),
					inputScope:      blackstart.NewInputFromValue("table"),
					inputSchema:     blackstart.NewInputFromValue("public"),
					inputResource:   blackstart.NewInputFromValue("orders"),
				},
			},
			wantErr: "comma-separated permissions are not supported",
		},
		{
			name: "rejects_invalid_table_permission",
			op: blackstart.Operation{
				Id:     "grant-validate-table-permission",
				Module: "postgres_grant",
				Inputs: map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
					inputRole:       blackstart.NewInputFromValue("app_user"),
					inputPermission: blackstart.NewInputFromValue("CONNECT"),
					inputScope:      blackstart.NewInputFromValue("table"),
					inputSchema:     blackstart.NewInputFromValue("public"),
					inputResource:   blackstart.NewInputFromValue("orders"),
				},
			},
			wantErr: "invalid permission",
		},
		{
			name: "accepts_all_privileges_alias_for_table_scope",
			op: blackstart.Operation{
				Id:     "grant-validate-all-privileges",
				Module: "postgres_grant",
				Inputs: map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
					inputRole:       blackstart.NewInputFromValue("app_user"),
					inputPermission: blackstart.NewInputFromValue("ALL PRIVILEGES"),
					inputScope:      blackstart.NewInputFromValue("table"),
					inputSchema:     blackstart.NewInputFromValue("public"),
					inputResource:   blackstart.NewInputFromValue("orders"),
				},
			},
		},
		{
			name: "accepts_type_usage_permission",
			op: blackstart.Operation{
				Id:     "grant-validate-type-permission",
				Module: "postgres_grant",
				Inputs: map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
					inputRole:       blackstart.NewInputFromValue("app_user"),
					inputPermission: blackstart.NewInputFromValue("USAGE"),
					inputScope:      blackstart.NewInputFromValue("type"),
					inputResource:   blackstart.NewInputFromValue("my_type"),
				},
			},
		},
		{
			name: "rejects_invalid_large_object_resource",
			op: blackstart.Operation{
				Id:     "grant-validate-large-object-resource",
				Module: "postgres_grant",
				Inputs: map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(&fakeConn{}),
					inputRole:       blackstart.NewInputFromValue("app_user"),
					inputPermission: blackstart.NewInputFromValue("SELECT"),
					inputScope:      blackstart.NewInputFromValue("large_object"),
					inputResource:   blackstart.NewInputFromValue("abc"),
				},
			},
			wantErr: "large object resource must be a positive integer loid",
		},
		{
			name: "rejects_with_grant_option_for_instance_scope",
			op: blackstart.Operation{
				Id:     "grant-validate-with-grant-option-instance",
				Module: "postgres_grant",
				Inputs: map[string]blackstart.Input{
					inputConnection:      blackstart.NewInputFromValue(&fakeConn{}),
					inputRole:            blackstart.NewInputFromValue("app_user"),
					inputPermission:      blackstart.NewInputFromValue("pg_monitor"),
					inputScope:           blackstart.NewInputFromValue("instance"),
					inputWithGrantOption: blackstart.NewInputFromValue(true),
				},
			},
			wantErr: "not supported when scope is INSTANCE",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.Validate(tt.op)
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

type fakeConn struct{}

func TestGrant(t *testing.T) {
	var err error
	ctx := context.Background()
	db, teardownPgInstance := createTestInstance(ctx, t)
	defer teardownPgInstance()

	tests := []struct {
		name        string
		setup       func(t *testing.T)
		checkResult bool
		grant       grantModule
		inputs      map[string]blackstart.Input
	}{
		{
			name: "schema_create_grant",
			setup: func(t *testing.T) {
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
			setup: func(t *testing.T) {
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
		{
			name: "table_select_update_grant",
			setup: func(t *testing.T) {
				_, err = db.Exec("CREATE ROLE blackstart_2")
				if err != nil {
					t.Fatalf("failed to create Role: %v", err)
				}
				_, err = db.Exec("CREATE TABLE IF NOT EXISTS public.blackstart_grant_test_orders (id INT)")
				if err != nil {
					t.Fatalf("failed to create table: %v", err)
				}
			},
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("blackstart_2"),
				inputPermission: blackstart.NewInputFromValue([]string{"SELECT", "UPDATE"}),
				inputSchema:     blackstart.NewInputFromValue("public"),
				inputResource:   blackstart.NewInputFromValue("blackstart_grant_test_orders"),
				inputScope:      blackstart.NewInputFromValue("table"),
				inputConnection: blackstart.NewInputFromValue(db),
			},
			grant: grantModule{},
		},
		{
			name: "table_multi_role_multi_permission_grant",
			setup: func(t *testing.T) {
				_, err = db.Exec("CREATE ROLE blackstart_3")
				if err != nil {
					t.Fatalf("failed to create Role: %v", err)
				}
				_, err = db.Exec("CREATE ROLE blackstart_4")
				if err != nil {
					t.Fatalf("failed to create Role: %v", err)
				}
				_, err = db.Exec("CREATE TABLE IF NOT EXISTS public.blackstart_grant_test_inventory (id INT)")
				if err != nil {
					t.Fatalf("failed to create table: %v", err)
				}
			},
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue([]string{"blackstart_3", "blackstart_4"}),
				inputPermission: blackstart.NewInputFromValue([]string{"SELECT", "UPDATE"}),
				inputSchema:     blackstart.NewInputFromValue("public"),
				inputResource:   blackstart.NewInputFromValue("blackstart_grant_test_inventory"),
				inputScope:      blackstart.NewInputFromValue("table"),
				inputConnection: blackstart.NewInputFromValue(db),
			},
			grant: grantModule{},
		},
		{
			name: "table_multi_schema_multi_resource_grant",
			setup: func(t *testing.T) {
				_, err = db.Exec("CREATE ROLE blackstart_5")
				if err != nil {
					t.Fatalf("failed to create Role: %v", err)
				}
				_, err = db.Exec("CREATE SCHEMA IF NOT EXISTS blackstart_schema_a")
				if err != nil {
					t.Fatalf("failed to create schema A: %v", err)
				}
				_, err = db.Exec("CREATE SCHEMA IF NOT EXISTS blackstart_schema_b")
				if err != nil {
					t.Fatalf("failed to create schema B: %v", err)
				}
				_, err = db.Exec("CREATE TABLE IF NOT EXISTS blackstart_schema_a.orders (id INT)")
				if err != nil {
					t.Fatalf("failed to create schema A table: %v", err)
				}
				_, err = db.Exec("CREATE TABLE IF NOT EXISTS blackstart_schema_b.orders (id INT)")
				if err != nil {
					t.Fatalf("failed to create schema B table: %v", err)
				}
				_, err = db.Exec("CREATE TABLE IF NOT EXISTS blackstart_schema_a.invoices (id INT)")
				if err != nil {
					t.Fatalf("failed to create schema A invoices table: %v", err)
				}
				_, err = db.Exec("CREATE TABLE IF NOT EXISTS blackstart_schema_b.invoices (id INT)")
				if err != nil {
					t.Fatalf("failed to create schema B invoices table: %v", err)
				}
			},
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("blackstart_5"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputSchema:     blackstart.NewInputFromValue([]string{"blackstart_schema_a", "blackstart_schema_b"}),
				inputResource:   blackstart.NewInputFromValue([]string{"orders", "invoices"}),
				inputScope:      blackstart.NewInputFromValue("table"),
				inputConnection: blackstart.NewInputFromValue(db),
			},
			grant: grantModule{},
		},
		{
			name: "sequence_usage_grant",
			setup: func(t *testing.T) {
				_, err = db.Exec("CREATE ROLE blackstart_7")
				if err != nil {
					t.Fatalf("failed to create Role: %v", err)
				}
				_, err = db.Exec(`CREATE SCHEMA IF NOT EXISTS blackstart_seq_schema`)
				if err != nil {
					t.Fatalf("failed to create schema: %v", err)
				}
				_, err = db.Exec(`CREATE SEQUENCE IF NOT EXISTS blackstart_seq_schema.blackstart_seq`)
				if err != nil {
					t.Fatalf("failed to create sequence: %v", err)
				}
			},
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("blackstart_7"),
				inputPermission: blackstart.NewInputFromValue("USAGE"),
				inputSchema:     blackstart.NewInputFromValue("blackstart_seq_schema"),
				inputResource:   blackstart.NewInputFromValue("blackstart_seq"),
				inputScope:      blackstart.NewInputFromValue("sequence"),
				inputConnection: blackstart.NewInputFromValue(db),
			},
			grant: grantModule{},
		},
		{
			name: "sequence_all_in_schema_grant",
			setup: func(t *testing.T) {
				_, err = db.Exec("CREATE ROLE blackstart_8")
				if err != nil {
					t.Fatalf("failed to create Role: %v", err)
				}
				_, err = db.Exec(`CREATE SCHEMA IF NOT EXISTS blackstart_seq_all_schema`)
				if err != nil {
					t.Fatalf("failed to create schema: %v", err)
				}
				_, err = db.Exec(`CREATE SEQUENCE IF NOT EXISTS blackstart_seq_all_schema.seq_a`)
				if err != nil {
					t.Fatalf("failed to create sequence seq_a: %v", err)
				}
				_, err = db.Exec(`CREATE SEQUENCE IF NOT EXISTS blackstart_seq_all_schema.seq_b`)
				if err != nil {
					t.Fatalf("failed to create sequence seq_b: %v", err)
				}
			},
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("blackstart_8"),
				inputPermission: blackstart.NewInputFromValue("USAGE"),
				inputSchema:     blackstart.NewInputFromValue("blackstart_seq_all_schema"),
				inputScope:      blackstart.NewInputFromValue("sequence"),
				inputAll:        blackstart.NewInputFromValue(true),
				inputConnection: blackstart.NewInputFromValue(db),
			},
			grant: grantModule{},
		},
		{
			name: "database_connect_grant",
			setup: func(t *testing.T) {
				_, err = db.Exec("CREATE ROLE blackstart_6")
				if err != nil {
					t.Fatalf("failed to create Role: %v", err)
				}
				_, err = db.Exec("CREATE DATABASE blackstart_grant_test_db")
				if err != nil {
					t.Fatalf("failed to create database: %v", err)
				}
				_, err = db.Exec("REVOKE CONNECT ON DATABASE blackstart_grant_test_db FROM PUBLIC")
				if err != nil {
					t.Fatalf("failed to revoke default connect from public: %v", err)
				}
			},
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("blackstart_6"),
				inputPermission: blackstart.NewInputFromValue("CONNECT"),
				inputResource:   blackstart.NewInputFromValue("blackstart_grant_test_db"),
				inputScope:      blackstart.NewInputFromValue("database"),
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
					tt.setup(t)
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

				// Revoke lifecycle for DoesNotExist path
				revokeCtx := blackstart.InputsToContext(ctx, tt.inputs, blackstart.DoesNotExistFlag)

				check, testErr = tt.grant.Check(revokeCtx)
				require.NoError(t, testErr)
				require.False(t, check)

				testErr = tt.grant.Set(revokeCtx)
				require.NoError(t, testErr)

				check, testErr = tt.grant.Check(revokeCtx)
				require.NoError(t, testErr)
				require.True(t, check)
			},
		)
	}

}

func TestGrant_WithGrantOptionLifecycle(t *testing.T) {
	ctx := context.Background()
	db, teardownPgInstance := createTestInstance(ctx, t)
	defer teardownPgInstance()

	tests := []struct {
		name   string
		setup  []string
		inputs map[string]blackstart.Input
	}{
		{
			name: "table_select_with_grant_option",
			setup: []string{
				`DROP ROLE IF EXISTS "blackstart_wgo_table_role";`,
				`CREATE ROLE "blackstart_wgo_table_role";`,
				`DROP SCHEMA IF EXISTS "blackstart_wgo_table_schema" CASCADE;`,
				`CREATE SCHEMA "blackstart_wgo_table_schema";`,
				`CREATE TABLE "blackstart_wgo_table_schema"."orders" (id INT);`,
			},
			inputs: map[string]blackstart.Input{
				inputConnection:      blackstart.NewInputFromValue(db),
				inputRole:            blackstart.NewInputFromValue("blackstart_wgo_table_role"),
				inputPermission:      blackstart.NewInputFromValue("SELECT"),
				inputScope:           blackstart.NewInputFromValue("TABLE"),
				inputSchema:          blackstart.NewInputFromValue("blackstart_wgo_table_schema"),
				inputResource:        blackstart.NewInputFromValue("orders"),
				inputWithGrantOption: blackstart.NewInputFromValue(true),
			},
		},
		{
			name: "sequence_usage_with_grant_option",
			setup: []string{
				`DROP ROLE IF EXISTS "blackstart_wgo_sequence_role";`,
				`CREATE ROLE "blackstart_wgo_sequence_role";`,
				`DROP SCHEMA IF EXISTS "blackstart_wgo_sequence_schema" CASCADE;`,
				`CREATE SCHEMA "blackstart_wgo_sequence_schema";`,
				`CREATE SEQUENCE "blackstart_wgo_sequence_schema"."orders_id_seq";`,
			},
			inputs: map[string]blackstart.Input{
				inputConnection:      blackstart.NewInputFromValue(db),
				inputRole:            blackstart.NewInputFromValue("blackstart_wgo_sequence_role"),
				inputPermission:      blackstart.NewInputFromValue("USAGE"),
				inputScope:           blackstart.NewInputFromValue("SEQUENCE"),
				inputSchema:          blackstart.NewInputFromValue("blackstart_wgo_sequence_schema"),
				inputResource:        blackstart.NewInputFromValue("orders_id_seq"),
				inputWithGrantOption: blackstart.NewInputFromValue(true),
			},
		},
		{
			name: "schema_usage_with_grant_option",
			setup: []string{
				`DROP ROLE IF EXISTS "blackstart_wgo_schema_role";`,
				`CREATE ROLE "blackstart_wgo_schema_role";`,
				`DROP SCHEMA IF EXISTS "blackstart_wgo_schema_target" CASCADE;`,
				`CREATE SCHEMA "blackstart_wgo_schema_target";`,
			},
			inputs: map[string]blackstart.Input{
				inputConnection:      blackstart.NewInputFromValue(db),
				inputRole:            blackstart.NewInputFromValue("blackstart_wgo_schema_role"),
				inputPermission:      blackstart.NewInputFromValue("USAGE"),
				inputScope:           blackstart.NewInputFromValue("SCHEMA"),
				inputResource:        blackstart.NewInputFromValue("blackstart_wgo_schema_target"),
				inputWithGrantOption: blackstart.NewInputFromValue(true),
			},
		},
		{
			name: "function_execute_with_grant_option",
			setup: []string{
				`DROP ROLE IF EXISTS "blackstart_wgo_function_role";`,
				`CREATE ROLE "blackstart_wgo_function_role";`,
				`DROP SCHEMA IF EXISTS "blackstart_wgo_function_schema" CASCADE;`,
				`CREATE SCHEMA "blackstart_wgo_function_schema";`,
				`CREATE OR REPLACE FUNCTION "blackstart_wgo_function_schema"."do_work"(x integer) RETURNS integer LANGUAGE sql AS $$ SELECT x + 1; $$;`,
				`REVOKE EXECUTE ON FUNCTION "blackstart_wgo_function_schema"."do_work"(integer) FROM PUBLIC;`,
			},
			inputs: map[string]blackstart.Input{
				inputConnection:      blackstart.NewInputFromValue(db),
				inputRole:            blackstart.NewInputFromValue("blackstart_wgo_function_role"),
				inputPermission:      blackstart.NewInputFromValue("EXECUTE"),
				inputScope:           blackstart.NewInputFromValue("FUNCTION"),
				inputSchema:          blackstart.NewInputFromValue("blackstart_wgo_function_schema"),
				inputResource:        blackstart.NewInputFromValue("do_work(integer)"),
				inputWithGrantOption: blackstart.NewInputFromValue(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for _, stmt := range tt.setup {
				_, err := db.ExecContext(ctx, stmt)
				require.NoError(t, err)
			}

			g := grantModule{}
			mctx := blackstart.InputsToContext(ctx, tt.inputs)

			ok, err := g.Check(mctx)
			require.NoError(t, err)
			require.False(t, ok)

			err = g.Set(mctx)
			require.NoError(t, err)

			ok, err = g.Check(mctx)
			require.NoError(t, err)
			require.True(t, ok)

			revokeCtx := blackstart.InputsToContext(ctx, tt.inputs, blackstart.DoesNotExistFlag)
			ok, err = g.Check(revokeCtx)
			require.NoError(t, err)
			require.False(t, ok)

			err = g.Set(revokeCtx)
			require.NoError(t, err)

			ok, err = g.Check(revokeCtx)
			require.NoError(t, err)
			require.True(t, ok)
		})
	}
}

func TestExpandGrantsFromContext_InvalidOptionalInputType(t *testing.T) {
	ctx := blackstart.InputsToContext(
		context.Background(),
		map[string]blackstart.Input{
			inputRole:       blackstart.NewInputFromValue("blackstart_0"),
			inputPermission: blackstart.NewInputFromValue("SELECT"),
			inputScope:      blackstart.NewInputFromValue("table"),
			inputSchema:     blackstart.NewInputFromValue(123),
		},
	)

	_, err := expandGrantsFromContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), `input "schema" is invalid`)
}

func TestExpandGrantsFromContext_SchemaScopeRejectsMismatchedSchemaResource(t *testing.T) {
	ctx := blackstart.InputsToContext(
		context.Background(),
		map[string]blackstart.Input{
			inputRole:       blackstart.NewInputFromValue("blackstart_0"),
			inputPermission: blackstart.NewInputFromValue("USAGE"),
			inputScope:      blackstart.NewInputFromValue("schema"),
			inputSchema:     blackstart.NewInputFromValue([]string{"schema_a", "schema_b"}),
			inputResource:   blackstart.NewInputFromValue([]string{"schema_a", "schema_c"}),
		},
	)

	_, err := expandGrantsFromContext(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), `must match for scope SCHEMA`)
}

func TestExpandGrantsFromContext_Combinations(t *testing.T) {
	tests := []struct {
		name     string
		inputs   map[string]blackstart.Input
		wantLen  int
		wantErr  string
		validate func(t *testing.T, grants []*grant)
	}{
		{
			name: "instance_multi_role_multi_permission",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue([]string{"role_a", "role_b"}),
				inputPermission: blackstart.NewInputFromValue([]string{"pg_read_all_data", "pg_write_all_data"}),
				inputScope:      blackstart.NewInputFromValue("instance"),
			},
			wantLen: 4,
			validate: func(t *testing.T, grants []*grant) {
				for _, g := range grants {
					require.Equal(t, "INSTANCE", g.Scope)
					require.Equal(t, "", g.Schema)
					require.Equal(t, "", g.Resource)
				}
			},
		},
		{
			name: "database_requires_resource",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("CONNECT"),
				inputScope:      blackstart.NewInputFromValue("database"),
			},
			wantErr: `input "resource" must be provided when scope is DATABASE`,
		},
		{
			name: "instance_scope_rejects_invalid_permission_role_identifier",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("bad\"role"),
				inputScope:      blackstart.NewInputFromValue("instance"),
			},
			wantErr: `invalid permission`,
		},
		{
			name: "database_scope_rejects_invalid_permission",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputResource:   blackstart.NewInputFromValue("postgres"),
				inputScope:      blackstart.NewInputFromValue("database"),
			},
			wantErr: `invalid permission`,
		},
		{
			name: "table_scope_rejects_invalid_permission",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("CONNECT"),
				inputSchema:     blackstart.NewInputFromValue("public"),
				inputResource:   blackstart.NewInputFromValue("orders"),
				inputScope:      blackstart.NewInputFromValue("table"),
			},
			wantErr: `invalid permission`,
		},
		{
			name: "rejects_comma_delimited_permission",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("SELECT,UPDATE"),
				inputSchema:     blackstart.NewInputFromValue("public"),
				inputResource:   blackstart.NewInputFromValue("orders"),
				inputScope:      blackstart.NewInputFromValue("table"),
			},
			wantErr: `comma-separated permissions are not supported`,
		},
		{
			name: "table_scope_accepts_all_privileges_alias",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("all privileges"),
				inputSchema:     blackstart.NewInputFromValue("public"),
				inputResource:   blackstart.NewInputFromValue("orders"),
				inputScope:      blackstart.NewInputFromValue("table"),
			},
			wantLen: 1,
			validate: func(t *testing.T, grants []*grant) {
				require.Equal(t, "ALL", grants[0].Permission)
			},
		},
		{
			name: "rejects_invalid_role_identifier",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("bad\"role"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputSchema:     blackstart.NewInputFromValue("public"),
				inputResource:   blackstart.NewInputFromValue("orders"),
				inputScope:      blackstart.NewInputFromValue("table"),
			},
			wantErr: `invalid role`,
		},
		{
			name: "schema_scope_uses_resource_target",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("USAGE"),
				inputScope:      blackstart.NewInputFromValue("schema"),
				inputSchema:     blackstart.NewInputFromValue([]string{"analytics"}),
			},
			wantLen: 1,
			validate: func(t *testing.T, grants []*grant) {
				require.Equal(t, "SCHEMA", grants[0].Scope)
				require.Equal(t, "", grants[0].Schema)
				require.Equal(t, "analytics", grants[0].Resource)
			},
		},
		{
			name: "table_scope_all_tables_multi_role_multi_permission",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue([]string{"role_a", "role_b"}),
				inputPermission: blackstart.NewInputFromValue([]string{"SELECT", "UPDATE"}),
				inputScope:      blackstart.NewInputFromValue("table"),
				inputSchema:     blackstart.NewInputFromValue([]string{"a", "b"}),
				inputAll:        blackstart.NewInputFromValue(true),
			},
			wantLen: 8,
			validate: func(t *testing.T, grants []*grant) {
				for _, g := range grants {
					require.Equal(t, "TABLE", g.Scope)
					require.True(t, g.All)
					require.NotEmpty(t, g.Schema)
					require.Empty(t, g.Resource)
				}
			},
		},
		{
			name: "function_scope_all_multi_schema",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("EXECUTE"),
				inputScope:      blackstart.NewInputFromValue("function"),
				inputSchema:     blackstart.NewInputFromValue([]string{"a", "b"}),
				inputAll:        blackstart.NewInputFromValue(true),
			},
			wantLen: 2,
			validate: func(t *testing.T, grants []*grant) {
				for _, g := range grants {
					require.Equal(t, "FUNCTION", g.Scope)
					require.True(t, g.All)
					require.NotEmpty(t, g.Schema)
					require.Empty(t, g.Resource)
				}
			},
		},
		{
			name: "procedure_scope_requires_schema",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("EXECUTE"),
				inputScope:      blackstart.NewInputFromValue("procedure"),
				inputResource:   blackstart.NewInputFromValue("do_work(integer)"),
			},
			wantErr: `input "schema" must be provided when scope is PROCEDURE`,
		},
		{
			name: "routine_scope_requires_resource_when_all_false",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("EXECUTE"),
				inputScope:      blackstart.NewInputFromValue("routine"),
				inputSchema:     blackstart.NewInputFromValue("public"),
			},
			wantErr: `input "resource" must be provided when scope is ROUTINE`,
		},
		{
			name: "table_scope_multi_schema_resource_cartesian",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputScope:      blackstart.NewInputFromValue("table"),
				inputSchema:     blackstart.NewInputFromValue([]string{"a", "b"}),
				inputResource:   blackstart.NewInputFromValue([]string{"orders", "invoices"}),
			},
			wantLen: 4,
			validate: func(t *testing.T, grants []*grant) {
				for _, g := range grants {
					require.Equal(t, "TABLE", g.Scope)
					require.NotEmpty(t, g.Schema)
					require.NotEmpty(t, g.Resource)
				}
			},
		},
		{
			name: "sequence_scope_all_multi_schema",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue([]string{"USAGE", "SELECT"}),
				inputScope:      blackstart.NewInputFromValue("sequence"),
				inputSchema:     blackstart.NewInputFromValue([]string{"a", "b"}),
				inputAll:        blackstart.NewInputFromValue(true),
			},
			wantLen: 4,
			validate: func(t *testing.T, grants []*grant) {
				for _, g := range grants {
					require.Equal(t, "SEQUENCE", g.Scope)
					require.True(t, g.All)
					require.NotEmpty(t, g.Schema)
					require.Empty(t, g.Resource)
				}
			},
		},
		{
			name: "sequence_scope_requires_resource_when_all_false",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("USAGE"),
				inputScope:      blackstart.NewInputFromValue("sequence"),
				inputSchema:     blackstart.NewInputFromValue("public"),
			},
			wantErr: `input "resource" must be provided when scope is SEQUENCE`,
		},
		{
			name: "domain_scope_requires_resource",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("USAGE"),
				inputScope:      blackstart.NewInputFromValue("domain"),
			},
			wantErr: `input "resource" must be provided when scope is DOMAIN`,
		},
		{
			name: "domain_scope_rejects_schema_input",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("USAGE"),
				inputScope:      blackstart.NewInputFromValue("domain"),
				inputSchema:     blackstart.NewInputFromValue("public"),
				inputResource:   blackstart.NewInputFromValue("my_domain"),
			},
			wantErr: `input "schema" is not supported when scope is DOMAIN`,
		},
		{
			name: "large_object_scope_requires_numeric_resource",
			inputs: map[string]blackstart.Input{
				inputRole:       blackstart.NewInputFromValue("role_a"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputScope:      blackstart.NewInputFromValue("large_object"),
				inputResource:   blackstart.NewInputFromValue("not-a-loid"),
			},
			wantErr: `large object resource must be a positive integer loid`,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ctx := blackstart.InputsToContext(context.Background(), tt.inputs)
				grants, err := expandGrantsFromContext(ctx)
				if tt.wantErr != "" {
					require.Error(t, err)
					require.Contains(t, err.Error(), tt.wantErr)
					return
				}
				require.NoError(t, err)
				require.Len(t, grants, tt.wantLen)
				if tt.validate != nil {
					tt.validate(t, grants)
				}
			},
		)
	}
}

func TestGrantQueries_AllScopes_Render(t *testing.T) {
	ctx := context.Background()
	db, teardownPgInstance := createTestInstance(ctx, t)
	defer teardownPgInstance()

	tests := []struct {
		name           string
		target         *grant
		setupSQL       []string
		setContains    []string
		revokeContains []string
	}{
		{
			name: "instance",
			target: &grant{
				Role:       "blackstart_render_instance_target",
				Permission: "blackstart_render_instance_member",
				Scope:      "INSTANCE",
			},
			setupSQL: []string{
				`DROP ROLE IF EXISTS "blackstart_render_instance_target";`,
				`DROP ROLE IF EXISTS "blackstart_render_instance_member";`,
				`CREATE ROLE "blackstart_render_instance_member";`,
				`CREATE ROLE "blackstart_render_instance_target";`,
			},
			setContains:    []string{"GRANT", "TO"},
			revokeContains: []string{"REVOKE", "FROM"},
		},
		{
			name: "database",
			target: &grant{
				Role:       "blackstart_render_database_role",
				Permission: "CONNECT",
				Resource:   "blackstart_render_database",
				Scope:      "DATABASE",
			},
			setupSQL: []string{
				`DROP ROLE IF EXISTS "blackstart_render_database_role";`,
				`CREATE ROLE "blackstart_render_database_role";`,
				`DROP DATABASE IF EXISTS "blackstart_render_database";`,
				`CREATE DATABASE "blackstart_render_database";`,
				`REVOKE CONNECT ON DATABASE "blackstart_render_database" FROM PUBLIC;`,
			},
			setContains:    []string{"GRANT", "ON DATABASE", "blackstart_render_database"},
			revokeContains: []string{"REVOKE", "ON DATABASE", "blackstart_render_database"},
		},
		{
			name: "database_all_privileges_alias",
			target: &grant{
				Role:       "blackstart_render_database_all_role",
				Permission: "ALL PRIVILEGES",
				Resource:   "blackstart_render_database_all",
				Scope:      "DATABASE",
			},
			setupSQL: []string{
				`DROP ROLE IF EXISTS "blackstart_render_database_all_role";`,
				`CREATE ROLE "blackstart_render_database_all_role";`,
				`DROP DATABASE IF EXISTS "blackstart_render_database_all";`,
				`CREATE DATABASE "blackstart_render_database_all";`,
			},
			setContains:    []string{"GRANT", "ON DATABASE", "blackstart_render_database_all"},
			revokeContains: []string{"REVOKE", "ON DATABASE", "blackstart_render_database_all"},
		},
		{
			name: "schema",
			target: &grant{
				Role:       "blackstart_render_schema_role",
				Permission: "USAGE",
				Resource:   "blackstart_render_schema",
				Scope:      "SCHEMA",
			},
			setupSQL: []string{
				`DROP ROLE IF EXISTS "blackstart_render_schema_role";`,
				`CREATE ROLE "blackstart_render_schema_role";`,
				`DROP SCHEMA IF EXISTS "blackstart_render_schema" CASCADE;`,
				`CREATE SCHEMA "blackstart_render_schema";`,
			},
			setContains:    []string{"GRANT", "ON SCHEMA", "blackstart_render_schema"},
			revokeContains: []string{"REVOKE", "ON SCHEMA", "blackstart_render_schema"},
		},
		{
			name: "table_all_tables",
			target: &grant{
				Role:       "blackstart_render_all_tables_role",
				Permission: "SELECT",
				Schema:     "blackstart_render_all_tables_schema",
				Scope:      "TABLE",
				All:        true,
			},
			setupSQL: []string{
				`DROP ROLE IF EXISTS "blackstart_render_all_tables_role";`,
				`CREATE ROLE "blackstart_render_all_tables_role";`,
				`DROP SCHEMA IF EXISTS "blackstart_render_all_tables_schema" CASCADE;`,
				`CREATE SCHEMA "blackstart_render_all_tables_schema";`,
				`CREATE TABLE "blackstart_render_all_tables_schema"."orders" (id INT);`,
				`CREATE TABLE "blackstart_render_all_tables_schema"."invoices" (id INT);`,
			},
			setContains:    []string{"GRANT", "ON ALL TABLES IN SCHEMA", `"blackstart_render_all_tables_schema"`},
			revokeContains: []string{"REVOKE", "ON ALL TABLES IN SCHEMA", `"blackstart_render_all_tables_schema"`},
		},
		{
			name: "function",
			target: &grant{
				Role:       "blackstart_render_function_role",
				Permission: "EXECUTE",
				Schema:     "blackstart_render_function_schema",
				Resource:   "blackstart_add(integer)",
				Scope:      "FUNCTION",
			},
			setupSQL: []string{
				`DROP ROLE IF EXISTS "blackstart_render_function_role";`,
				`CREATE ROLE "blackstart_render_function_role";`,
				`DROP SCHEMA IF EXISTS "blackstart_render_function_schema" CASCADE;`,
				`CREATE SCHEMA "blackstart_render_function_schema";`,
				`CREATE OR REPLACE FUNCTION "blackstart_render_function_schema"."blackstart_add"(x integer) RETURNS integer LANGUAGE sql AS $$ SELECT x + 1; $$;`,
				`REVOKE EXECUTE ON FUNCTION "blackstart_render_function_schema"."blackstart_add"(integer) FROM PUBLIC;`,
			},
			setContains:    []string{"GRANT", "ON FUNCTION", `"blackstart_render_function_schema".blackstart_add(integer)`},
			revokeContains: []string{"REVOKE", "ON FUNCTION", `"blackstart_render_function_schema".blackstart_add(integer)`},
		},
		{
			name: "procedure_all_in_schema",
			target: &grant{
				Role:       "blackstart_render_procedure_all_role",
				Permission: "EXECUTE",
				Schema:     "blackstart_render_procedure_all_schema",
				Scope:      "PROCEDURE",
				All:        true,
			},
			setupSQL: []string{
				`DROP ROLE IF EXISTS "blackstart_render_procedure_all_role";`,
				`CREATE ROLE "blackstart_render_procedure_all_role";`,
				`DROP SCHEMA IF EXISTS "blackstart_render_procedure_all_schema" CASCADE;`,
				`CREATE SCHEMA "blackstart_render_procedure_all_schema";`,
				`CREATE OR REPLACE PROCEDURE "blackstart_render_procedure_all_schema"."proc_a"(x integer) LANGUAGE sql AS $$ SELECT x; $$;`,
				`CREATE OR REPLACE PROCEDURE "blackstart_render_procedure_all_schema"."proc_b"(x integer) LANGUAGE sql AS $$ SELECT x; $$;`,
				`REVOKE EXECUTE ON ALL PROCEDURES IN SCHEMA "blackstart_render_procedure_all_schema" FROM PUBLIC;`,
			},
			setContains:    []string{"GRANT", "ON ALL PROCEDURES IN SCHEMA", `"blackstart_render_procedure_all_schema"`},
			revokeContains: []string{"REVOKE", "ON ALL PROCEDURES IN SCHEMA", `"blackstart_render_procedure_all_schema"`},
		},
		{
			name: "routine_all_in_schema",
			target: &grant{
				Role:       "blackstart_render_routine_all_role",
				Permission: "EXECUTE",
				Schema:     "blackstart_render_routine_all_schema",
				Scope:      "ROUTINE",
				All:        true,
			},
			setupSQL: []string{
				`DROP ROLE IF EXISTS "blackstart_render_routine_all_role";`,
				`CREATE ROLE "blackstart_render_routine_all_role";`,
				`DROP SCHEMA IF EXISTS "blackstart_render_routine_all_schema" CASCADE;`,
				`CREATE SCHEMA "blackstart_render_routine_all_schema";`,
				`CREATE OR REPLACE FUNCTION "blackstart_render_routine_all_schema"."fn_a"(x integer) RETURNS integer LANGUAGE sql AS $$ SELECT x + 1; $$;`,
				`CREATE OR REPLACE PROCEDURE "blackstart_render_routine_all_schema"."proc_a"(x integer) LANGUAGE sql AS $$ SELECT x; $$;`,
				`REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA "blackstart_render_routine_all_schema" FROM PUBLIC;`,
				`REVOKE EXECUTE ON ALL PROCEDURES IN SCHEMA "blackstart_render_routine_all_schema" FROM PUBLIC;`,
			},
			setContains:    []string{"GRANT", "ON ALL ROUTINES IN SCHEMA", `"blackstart_render_routine_all_schema"`},
			revokeContains: []string{"REVOKE", "ON ALL ROUTINES IN SCHEMA", `"blackstart_render_routine_all_schema"`},
		},
		{
			name: "table",
			target: &grant{
				Role:       "blackstart_render_table_role",
				Permission: "SELECT",
				Schema:     "blackstart_render_table_schema",
				Resource:   "blackstart_render_orders",
				Scope:      "TABLE",
			},
			setupSQL: []string{
				`DROP ROLE IF EXISTS "blackstart_render_table_role";`,
				`CREATE ROLE "blackstart_render_table_role";`,
				`DROP SCHEMA IF EXISTS "blackstart_render_table_schema" CASCADE;`,
				`CREATE SCHEMA "blackstart_render_table_schema";`,
				`CREATE TABLE "blackstart_render_table_schema"."blackstart_render_orders" (id INT);`,
			},
			setContains:    []string{"GRANT", "ON TABLE", `"blackstart_render_table_schema"."blackstart_render_orders"`},
			revokeContains: []string{"REVOKE", "ON TABLE", `"blackstart_render_table_schema"."blackstart_render_orders"`},
		},
		{
			name: "sequence",
			target: &grant{
				Role:       "blackstart_render_sequence_role",
				Permission: "USAGE",
				Schema:     "blackstart_render_sequence_schema",
				Resource:   "blackstart_render_sequence",
				Scope:      "SEQUENCE",
			},
			setupSQL: []string{
				`DROP ROLE IF EXISTS "blackstart_render_sequence_role";`,
				`CREATE ROLE "blackstart_render_sequence_role";`,
				`DROP SCHEMA IF EXISTS "blackstart_render_sequence_schema" CASCADE;`,
				`CREATE SCHEMA "blackstart_render_sequence_schema";`,
				`CREATE SEQUENCE "blackstart_render_sequence_schema"."blackstart_render_sequence";`,
			},
			setContains:    []string{"GRANT", "ON SEQUENCE", `"blackstart_render_sequence_schema"."blackstart_render_sequence"`},
			revokeContains: []string{"REVOKE", "ON SEQUENCE", `"blackstart_render_sequence_schema"."blackstart_render_sequence"`},
		},
		{
			name: "sequence_all_in_schema",
			target: &grant{
				Role:       "blackstart_render_sequence_all_role",
				Permission: "USAGE",
				Schema:     "blackstart_render_sequence_all_schema",
				Scope:      "SEQUENCE",
				All:        true,
			},
			setupSQL: []string{
				`DROP ROLE IF EXISTS "blackstart_render_sequence_all_role";`,
				`CREATE ROLE "blackstart_render_sequence_all_role";`,
				`DROP SCHEMA IF EXISTS "blackstart_render_sequence_all_schema" CASCADE;`,
				`CREATE SCHEMA "blackstart_render_sequence_all_schema";`,
				`CREATE SEQUENCE "blackstart_render_sequence_all_schema"."seq_a";`,
				`CREATE SEQUENCE "blackstart_render_sequence_all_schema"."seq_b";`,
			},
			setContains:    []string{"GRANT", "ON ALL SEQUENCES IN SCHEMA", `"blackstart_render_sequence_all_schema"`},
			revokeContains: []string{"REVOKE", "ON ALL SEQUENCES IN SCHEMA", `"blackstart_render_sequence_all_schema"`},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				for _, setupStmt := range tt.setupSQL {
					_, err := db.ExecContext(ctx, setupStmt)
					require.NoError(t, err)
				}

				setQuery, setParams, err := getGrantSetQuery(tt.target)
				require.NoError(t, err)
				for _, c := range tt.setContains {
					require.Contains(t, setQuery, c)
				}

				_, err = db.ExecContext(ctx, setQuery, setParams...)
				require.NoError(t, err)

				existsQuery, existsParams, err := getGrantExistsQuery(tt.target)
				require.NoError(t, err)

				var exists bool
				err = db.QueryRowContext(ctx, existsQuery, existsParams...).Scan(&exists)
				require.NoError(t, err)
				require.True(t, exists)

				revokeQuery, revokeParams, err := getGrantRevokeQuery(tt.target)
				require.NoError(t, err)
				for _, c := range tt.revokeContains {
					require.Contains(t, revokeQuery, c)
				}

				_, err = db.ExecContext(ctx, revokeQuery, revokeParams...)
				require.NoError(t, err)

				err = db.QueryRowContext(ctx, existsQuery, existsParams...).Scan(&exists)
				require.NoError(t, err)
				require.False(t, exists)
			},
		)
	}
}

func TestGrantQueries_RevokeRejectsInvalidPermissions(t *testing.T) {
	tests := []struct {
		name   string
		target *grant
	}{
		{
			name: "database_invalid_permission",
			target: &grant{
				Role:       "role_a",
				Permission: "SELECT",
				Resource:   "postgres",
				Scope:      "DATABASE",
			},
		},
		{
			name: "schema_invalid_permission",
			target: &grant{
				Role:       "role_a",
				Permission: "SELECT",
				Resource:   "public",
				Scope:      "SCHEMA",
			},
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				_, _, err := getGrantRevokeQuery(tt.target)
				require.Error(t, err)
				require.Contains(t, err.Error(), "invalid")
			},
		)
	}
}

func TestGrantQueryRendering_QuotedIdentifierBoundaries(t *testing.T) {
	t.Run("database_scope_keeps_permission_unquoted_and_identifiers_quoted", func(t *testing.T) {
		target := &grant{
			Role:       "iam-role@appomni-demo.iam",
			Permission: "CONNECT",
			Resource:   "app-db-prod",
			Scope:      "DATABASE",
		}

		query, _, err := getGrantSetQuery(target)
		require.NoError(t, err)
		require.Contains(t, query, `GRANT CONNECT ON DATABASE "app-db-prod" TO "iam-role@appomni-demo.iam";`)
		require.NotContains(t, query, `"CONNECT"`)
	})

	t.Run("table_scope_keeps_permission_unquoted_and_identifiers_quoted", func(t *testing.T) {
		target := &grant{
			Role:       "iam-role@appomni-demo.iam",
			Permission: "SELECT",
			Schema:     "orders-api",
			Resource:   "daily-rollup",
			Scope:      "TABLE",
		}

		query, _, err := getGrantSetQuery(target)
		require.NoError(t, err)
		require.Contains(
			t,
			query,
			`GRANT SELECT ON TABLE "orders-api"."daily-rollup" TO "iam-role@appomni-demo.iam";`,
		)
		require.NotContains(t, query, `"SELECT"`)
	})

	t.Run("table_scope_with_grant_option_appends_clause", func(t *testing.T) {
		target := &grant{
			Role:            "iam-role@appomni-demo.iam",
			Permission:      "SELECT",
			Schema:          "orders-api",
			Resource:        "daily-rollup",
			Scope:           "TABLE",
			WithGrantOption: true,
		}

		query, _, err := getGrantSetQuery(target)
		require.NoError(t, err)
		require.Contains(
			t,
			query,
			`GRANT SELECT ON TABLE "orders-api"."daily-rollup" TO "iam-role@appomni-demo.iam" WITH GRANT OPTION;`,
		)
	})

	t.Run("non_instance_scope_rejects_identifier_like_permission", func(t *testing.T) {
		target := &grant{
			Role:       "iam-role@appomni-demo.iam",
			Permission: "pg-read-role@appomni-demo.iam",
			Resource:   "app-db-prod",
			Scope:      "DATABASE",
		}

		_, _, err := getGrantSetQuery(target)
		require.Error(t, err)
		require.Contains(t, err.Error(), "invalid")
	})
}

func TestGrantQueries_NewScopes_RenderOnly(t *testing.T) {
	tests := []struct {
		name           string
		target         *grant
		setContains    []string
		revokeContains []string
	}{
		{
			name: "domain",
			target: &grant{
				Role:       "role_a",
				Permission: "USAGE",
				Resource:   "domain_name",
				Scope:      "DOMAIN",
			},
			setContains:    []string{"GRANT USAGE ON DOMAIN", `"domain_name"`},
			revokeContains: []string{"REVOKE USAGE ON DOMAIN", `"domain_name"`},
		},
		{
			name: "fdw",
			target: &grant{
				Role:       "role_a",
				Permission: "USAGE",
				Resource:   "fdw_name",
				Scope:      "FDW",
			},
			setContains:    []string{"GRANT USAGE ON FOREIGN DATA WRAPPER", `"fdw_name"`},
			revokeContains: []string{"REVOKE USAGE ON FOREIGN DATA WRAPPER", `"fdw_name"`},
		},
		{
			name: "foreign_server",
			target: &grant{
				Role:       "role_a",
				Permission: "USAGE",
				Resource:   "server_name",
				Scope:      "FOREIGN_SERVER",
			},
			setContains:    []string{"GRANT USAGE ON FOREIGN SERVER", `"server_name"`},
			revokeContains: []string{"REVOKE USAGE ON FOREIGN SERVER", `"server_name"`},
		},
		{
			name: "language",
			target: &grant{
				Role:       "role_a",
				Permission: "USAGE",
				Resource:   "plpgsql",
				Scope:      "LANGUAGE",
			},
			setContains:    []string{"GRANT USAGE ON LANGUAGE", `"plpgsql"`},
			revokeContains: []string{"REVOKE USAGE ON LANGUAGE", `"plpgsql"`},
		},
		{
			name: "large_object",
			target: &grant{
				Role:       "role_a",
				Permission: "SELECT",
				Resource:   "12345",
				Scope:      "LARGE_OBJECT",
			},
			setContains:    []string{"GRANT SELECT ON LARGE OBJECT 12345"},
			revokeContains: []string{"REVOKE SELECT ON LARGE OBJECT 12345"},
		},
		{
			name: "parameter",
			target: &grant{
				Role:       "role_a",
				Permission: "SET",
				Resource:   "work_mem",
				Scope:      "PARAMETER",
			},
			setContains:    []string{"GRANT SET ON PARAMETER", `"work_mem"`},
			revokeContains: []string{"REVOKE SET ON PARAMETER", `"work_mem"`},
		},
		{
			name: "tablespace",
			target: &grant{
				Role:       "role_a",
				Permission: "CREATE",
				Resource:   "ts_data",
				Scope:      "TABLESPACE",
			},
			setContains:    []string{"GRANT CREATE ON TABLESPACE", `"ts_data"`},
			revokeContains: []string{"REVOKE CREATE ON TABLESPACE", `"ts_data"`},
		},
		{
			name: "type",
			target: &grant{
				Role:       "role_a",
				Permission: "USAGE",
				Resource:   "status_type",
				Scope:      "TYPE",
			},
			setContains:    []string{"GRANT USAGE ON TYPE", `"status_type"`},
			revokeContains: []string{"REVOKE USAGE ON TYPE", `"status_type"`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setQuery, _, err := getGrantSetQuery(tt.target)
			require.NoError(t, err)
			for _, c := range tt.setContains {
				require.Contains(t, setQuery, c)
			}

			revokeQuery, _, err := getGrantRevokeQuery(tt.target)
			require.NoError(t, err)
			for _, c := range tt.revokeContains {
				require.Contains(t, revokeQuery, c)
			}

			_, existsParams, err := getGrantExistsQuery(tt.target)
			require.NoError(t, err)
			require.NotEmpty(t, existsParams)
		})
	}
}
