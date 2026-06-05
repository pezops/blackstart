package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

// TestGrantValidate verifies static validation for MySQL grant inputs.
func TestGrantValidate(t *testing.T) {
	m := grantModule{}
	conn := &sql.DB{}

	tests := []struct {
		name    string
		inputs  map[string]blackstart.Input
		wantErr string
	}{
		{
			name: "missing_role",
			inputs: map[string]blackstart.Input{
				inputConnection: blackstart.NewInputFromValue(conn),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
			},
			wantErr: "missing required parameter: role",
		},
		{
			name: "invalid_scope",
			inputs: map[string]blackstart.Input{
				inputConnection: blackstart.NewInputFromValue(conn),
				inputRole:       blackstart.NewInputFromValue("app"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputScope:      blackstart.NewInputFromValue("schema"),
			},
			wantErr: "invalid scope",
		},
		{
			name: "invalid_permission",
			inputs: map[string]blackstart.Input{
				inputConnection: blackstart.NewInputFromValue(conn),
				inputRole:       blackstart.NewInputFromValue("app"),
				inputPermission: blackstart.NewInputFromValue("CONNECT"),
				inputScope:      blackstart.NewInputFromValue("TABLE"),
				inputSchema:     blackstart.NewInputFromValue("appdb"),
				inputResource:   blackstart.NewInputFromValue("orders"),
			},
			wantErr: "invalid permission",
		},
		{
			name: "all_requires_table_scope",
			inputs: map[string]blackstart.Input{
				inputConnection: blackstart.NewInputFromValue(conn),
				inputRole:       blackstart.NewInputFromValue("app"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputScope:      blackstart.NewInputFromValue("DATABASE"),
				inputResource:   blackstart.NewInputFromValue("appdb"),
				inputAll:        blackstart.NewInputFromValue(true),
			},
			wantErr: "only supported when scope is TABLE",
		},
		{
			name: "all_rejects_resource",
			inputs: map[string]blackstart.Input{
				inputConnection: blackstart.NewInputFromValue(conn),
				inputRole:       blackstart.NewInputFromValue("app"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputScope:      blackstart.NewInputFromValue("TABLE"),
				inputSchema:     blackstart.NewInputFromValue("appdb"),
				inputResource:   blackstart.NewInputFromValue("orders"),
				inputAll:        blackstart.NewInputFromValue(true),
			},
			wantErr: "must be empty when all is true",
		},
		{
			name: "invalid_identifier",
			inputs: map[string]blackstart.Input{
				inputConnection: blackstart.NewInputFromValue(conn),
				inputRole:       blackstart.NewInputFromValue("app"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputScope:      blackstart.NewInputFromValue("TABLE"),
				inputSchema:     blackstart.NewInputFromValue("bad`schema"),
				inputResource:   blackstart.NewInputFromValue("orders"),
			},
			wantErr: "invalid character",
		},
		{
			name: "invalid_account",
			inputs: map[string]blackstart.Input{
				inputConnection: blackstart.NewInputFromValue(conn),
				inputRole:       blackstart.NewInputFromValue("bad@host@again"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputScope:      blackstart.NewInputFromValue("DATABASE"),
				inputResource:   blackstart.NewInputFromValue("appdb"),
			},
			wantErr: "account must be formatted",
		},
		{
			name: "valid_database",
			inputs: map[string]blackstart.Input{
				inputConnection: blackstart.NewInputFromValue(conn),
				inputRole:       blackstart.NewInputFromValue("app"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputScope:      blackstart.NewInputFromValue("DATABASE"),
				inputResource:   blackstart.NewInputFromValue("appdb"),
			},
		},
		{
			name: "valid_table_all",
			inputs: map[string]blackstart.Input{
				inputConnection: blackstart.NewInputFromValue(conn),
				inputRole:       blackstart.NewInputFromValue("app"),
				inputPermission: blackstart.NewInputFromValue("SELECT"),
				inputScope:      blackstart.NewInputFromValue("TABLE"),
				inputSchema:     blackstart.NewInputFromValue("appdb"),
				inputAll:        blackstart.NewInputFromValue(true),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := m.Validate(blackstart.Operation{
				Id:     "grant-validate",
				Module: "mysql_grant",
				Inputs: tt.inputs,
			})
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

// TestGrantLifecycle verifies MySQL grant and revoke behavior against a real database.
func TestGrantLifecycle(t *testing.T) {
	ctx := context.Background()
	db, closeDb := createRootTestInstance(ctx, t)
	defer closeDb()

	tests := []struct {
		name    string
		role    string
		dbName  string
		inputs  func(*sql.DB, string, string) map[string]blackstart.Input
		prepare func(*testing.T, *sql.DB, string)
	}{
		{
			name:   "database_select",
			role:   "blackstart_db_select",
			dbName: "blackstart_grant_db",
			inputs: func(db *sql.DB, role string, dbName string) map[string]blackstart.Input {
				return map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(db),
					inputRole:       blackstart.NewInputFromValue(role),
					inputPermission: blackstart.NewInputFromValue("SELECT"),
					inputScope:      blackstart.NewInputFromValue("DATABASE"),
					inputResource:   blackstart.NewInputFromValue(dbName),
				}
			},
		},
		{
			name:   "table_select_update",
			role:   "blackstart_table_select",
			dbName: "blackstart_grant_table",
			inputs: func(db *sql.DB, role string, dbName string) map[string]blackstart.Input {
				return map[string]blackstart.Input{
					inputConnection: blackstart.NewInputFromValue(db),
					inputRole:       blackstart.NewInputFromValue(role),
					inputPermission: blackstart.NewInputFromValue([]string{"SELECT", "UPDATE"}),
					inputScope:      blackstart.NewInputFromValue("TABLE"),
					inputSchema:     blackstart.NewInputFromValue(dbName),
					inputResource:   blackstart.NewInputFromValue("orders"),
				}
			},
			prepare: func(t *testing.T, db *sql.DB, dbName string) {
				execMySQL(t, db, fmt.Sprintf("CREATE TABLE `%s`.`orders` (id INT PRIMARY KEY)", dbName))
			},
		},
		{
			name:   "all_tables_select_with_grant_option",
			role:   "blackstart_all_tables",
			dbName: "blackstart_grant_all",
			inputs: func(db *sql.DB, role string, dbName string) map[string]blackstart.Input {
				return map[string]blackstart.Input{
					inputConnection:      blackstart.NewInputFromValue(db),
					inputRole:            blackstart.NewInputFromValue(role),
					inputPermission:      blackstart.NewInputFromValue("SELECT"),
					inputScope:           blackstart.NewInputFromValue("TABLE"),
					inputSchema:          blackstart.NewInputFromValue(dbName),
					inputAll:             blackstart.NewInputFromValue(true),
					inputWithGrantOption: blackstart.NewInputFromValue(true),
				}
			},
			prepare: func(t *testing.T, db *sql.DB, dbName string) {
				execMySQL(t, db, fmt.Sprintf("CREATE TABLE `%s`.`orders` (id INT PRIMARY KEY)", dbName))
				execMySQL(t, db, fmt.Sprintf("CREATE TABLE `%s`.`customers` (id INT PRIMARY KEY)", dbName))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prepareGrantFixture(t, db, tt.dbName, tt.role)
			defer cleanupGrantFixture(t, db, tt.dbName, tt.role)
			if tt.prepare != nil {
				tt.prepare(t, db, tt.dbName)
			}

			mod := grantModule{}
			inputs := tt.inputs(db, tt.role, tt.dbName)
			require.NoError(t, mod.Validate(blackstart.Operation{Id: "grant", Module: "mysql_grant", Inputs: inputs}))

			checkCtx := grantTestContext(ctx, inputs, false)
			t.Log("checking grant before create")
			ok, err := mod.Check(checkCtx)
			require.NoError(t, err)
			require.False(t, ok)

			t.Log("creating grant")
			require.NoError(t, mod.Set(checkCtx))

			t.Log("checking grant after create")
			ok, err = mod.Check(checkCtx)
			require.NoError(t, err)
			require.True(t, ok)

			deleteCtx := grantTestContext(ctx, inputs, true)
			t.Log("checking grant before revoke")
			ok, err = mod.Check(deleteCtx)
			require.NoError(t, err)
			require.False(t, ok)

			t.Log("revoking grant")
			require.NoError(t, mod.Set(deleteCtx))

			t.Log("checking grant after revoke")
			ok, err = mod.Check(deleteCtx)
			require.NoError(t, err)
			require.True(t, ok)

			t.Log("checking normal grant after revoke")
			ok, err = mod.Check(checkCtx)
			require.NoError(t, err)
			require.False(t, ok)
		})
	}
}

// TestExpandGrantsFromContext verifies cross-product expansion of list-valued inputs.
func TestExpandGrantsFromContext(t *testing.T) {
	inputs := map[string]blackstart.Input{
		inputRole:       blackstart.NewInputFromValue([]string{"app", "reporting"}),
		inputPermission: blackstart.NewInputFromValue([]string{"select", "update"}),
		inputScope:      blackstart.NewInputFromValue("TABLE"),
		inputSchema:     blackstart.NewInputFromValue("appdb"),
		inputResource:   blackstart.NewInputFromValue([]string{"orders", "customers"}),
	}
	ctx := grantTestContext(context.Background(), inputs, false)

	targets, err := expandGrantsFromContext(ctx)
	require.NoError(t, err)
	require.Len(t, targets, 8)
	require.Equal(t, "SELECT", targets[0].Permission)
	require.Equal(t, scopes.table, targets[0].Scope)
}

// createRootTestInstance opens a root database connection to the shared MySQL test container.
func createRootTestInstance(ctx context.Context, t *testing.T) (*sql.DB, func()) {
	dsn, err := testMySQLDSN(ctx, "root", testMySQLRootPassword, testMySQLDatabase)
	require.NoError(t, err)

	db, err := sql.Open("mysql", dsn)
	require.NoError(t, err)
	closeDb := func() {
		if closeErr := db.Close(); closeErr != nil {
			t.Logf("failed to close root database connection: %v", closeErr)
		}
	}
	return db, closeDb
}

// grantTestContext creates a module context for a MySQL grant operation.
func grantTestContext(ctx context.Context, inputs map[string]blackstart.Input, doesNotExist bool) blackstart.ModuleContext {
	op := &blackstart.Operation{
		Id:           "grant",
		Module:       "mysql_grant",
		Inputs:       inputs,
		DoesNotExist: doesNotExist,
	}
	return blackstart.OpContext(ctx, op)
}

// prepareGrantFixture recreates the database and user used by a grant test.
func prepareGrantFixture(t *testing.T, db *sql.DB, dbName string, role string) {
	cleanupGrantFixture(t, db, dbName, role)
	execMySQL(t, db, fmt.Sprintf("CREATE DATABASE `%s`", dbName))
	execMySQL(t, db, fmt.Sprintf("CREATE USER '%s'@'%%' IDENTIFIED BY 'password'", role))
}

// cleanupGrantFixture removes the database and user used by a grant test.
func cleanupGrantFixture(t *testing.T, db *sql.DB, dbName string, role string) {
	execMySQL(t, db, fmt.Sprintf("DROP DATABASE IF EXISTS `%s`", dbName))
	execMySQL(t, db, fmt.Sprintf("DROP USER IF EXISTS '%s'@'%%'", role))
}

// execMySQL executes a test SQL statement and fails the test on error.
func execMySQL(t *testing.T, db *sql.DB, stmt string) {
	t.Helper()
	_, err := db.Exec(stmt)
	require.NoError(t, err)
}
