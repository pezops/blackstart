package cloudsql

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/api/sqladmin/v1"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

// TestDatabaseValidate verifies static input validation and permits runtime inputs.
func TestDatabaseValidate(t *testing.T) {
	tests := map[string]struct {
		configure func(*blackstart.Operation)
		wantErr   string
	}{
		"valid": {
			configure: func(*blackstart.Operation) {},
		},
		"dependency instance": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputInstance] = blackstart.NewInputFromDep("create-instance", "name")
			},
		},
		"dependency database": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputDatabase] = blackstart.NewInputFromDep("database-name", "database")
			},
		},
		"mysql options": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputCharset] = blackstart.NewInputFromValue("utf8mb4")
				op.Inputs[inputCollation] = blackstart.NewInputFromValue("utf8mb4_0900_ai_ci")
			},
		},
		"missing instance": {
			configure: func(op *blackstart.Operation) {
				delete(op.Inputs, inputInstance)
			},
			wantErr: "missing required parameter: instance",
		},
		"missing database": {
			configure: func(op *blackstart.Operation) {
				delete(op.Inputs, inputDatabase)
			},
			wantErr: "missing required parameter: database",
		},
		"invalid instance type": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputInstance] = blackstart.NewInputFromValue(map[string]string{"name": "instance"})
			},
			wantErr: "invalid instance:",
		},
		"empty instance": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputInstance] = blackstart.NewInputFromValue("")
			},
			wantErr: "invalid instance: value cannot be empty",
		},
		"invalid database type": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputDatabase] = blackstart.NewInputFromValue([]string{"app"})
			},
			wantErr: "invalid database:",
		},
		"empty database": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputDatabase] = blackstart.NewInputFromValue("")
			},
			wantErr: "invalid database: value cannot be empty",
		},
		"invalid charset type": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputCharset] = blackstart.NewInputFromValue(map[string]string{"charset": "utf8mb4"})
			},
			wantErr: "invalid charset:",
		},
		"invalid collation type": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputCollation] = blackstart.NewInputFromValue([]string{"utf8mb4_0900_ai_ci"})
			},
			wantErr: "invalid collation:",
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				op := testCloudSQLDatabaseOperation("app")
				tt.configure(&op)

				err := (&database{}).Validate(op)
				if tt.wantErr == "" {
					require.NoError(t, err)
					return
				}
				require.ErrorContains(t, err, tt.wantErr)
			},
		)
	}
}

// TestDatabaseCheckWithFakeAdminAPI verifies database checks against the fake Admin API.
func TestDatabaseCheckWithFakeAdminAPI(t *testing.T) {
	tests := map[string]struct {
		databases    []*sqladmin.Database
		doesNotExist bool
		want         bool
	}{
		"existing database": {
			databases: []*sqladmin.Database{{Name: "app", Instance: "instance", Project: "project"}},
			want:      true,
		},
		"missing database": {},
		"missing database satisfies does not exist": {
			doesNotExist: true,
			want:         true,
		},
		"existing database fails does not exist": {
			databases:    []*sqladmin.Database{{Name: "app", Instance: "instance", Project: "project"}},
			doesNotExist: true,
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, "POSTGRES_17")
				api.databases = cloneDatabases(tt.databases)
				op := testCloudSQLDatabaseOperation("app")
				op.DoesNotExist = tt.doesNotExist
				ctx := blackstart.OpContext(context.Background(), &op)

				got, err := (&database{runtime: api.runtime(nil)}).Check(ctx)
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
			},
		)
	}
}

// TestDatabaseSetWithFakeAdminAPI verifies database create and delete calls against the fake Admin API.
func TestDatabaseSetWithFakeAdminAPI(t *testing.T) {
	tests := map[string]struct {
		version       string
		databases     []*sqladmin.Database
		configure     func(*blackstart.Operation)
		doesNotExist  bool
		wantDatabases []*sqladmin.Database
		wantInsert    int
		wantDelete    int
		assertRequest func(*testing.T, *fakeCloudSQLAdmin)
	}{
		"creates postgres database": {
			version:       "POSTGRES_17",
			wantDatabases: []*sqladmin.Database{{Name: "app", Instance: "instance", Project: "project"}},
			wantInsert:    1,
			assertRequest: func(t *testing.T, api *fakeCloudSQLAdmin) {
				t.Helper()
				require.Equal(t, "app", api.insertedDatabases[0].Name)
				require.Empty(t, api.insertedDatabases[0].Charset)
				require.Empty(t, api.insertedDatabases[0].Collation)
			},
		},
		"creates mysql database with options": {
			version: "MYSQL_8_4",
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputCharset] = blackstart.NewInputFromValue("utf8mb4")
				op.Inputs[inputCollation] = blackstart.NewInputFromValue("utf8mb4_0900_ai_ci")
			},
			wantDatabases: []*sqladmin.Database{
				{
					Name:      "app",
					Instance:  "instance",
					Project:   "project",
					Charset:   "utf8mb4",
					Collation: "utf8mb4_0900_ai_ci",
				},
			},
			wantInsert: 1,
			assertRequest: func(t *testing.T, api *fakeCloudSQLAdmin) {
				t.Helper()
				require.Equal(t, "utf8mb4", api.insertedDatabases[0].Charset)
				require.Equal(t, "utf8mb4_0900_ai_ci", api.insertedDatabases[0].Collation)
			},
		},
		"creates mysql database without options": {
			version:       "MYSQL_8_4",
			wantDatabases: []*sqladmin.Database{{Name: "app", Instance: "instance", Project: "project"}},
			wantInsert:    1,
			assertRequest: func(t *testing.T, api *fakeCloudSQLAdmin) {
				t.Helper()
				require.Empty(t, api.insertedDatabases[0].Charset)
				require.Empty(t, api.insertedDatabases[0].Collation)
			},
		},
		"deletes existing database": {
			version:       "POSTGRES_17",
			databases:     []*sqladmin.Database{{Name: "app", Instance: "instance", Project: "project"}},
			doesNotExist:  true,
			wantDatabases: []*sqladmin.Database{},
			wantDelete:    1,
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, tt.version)
				api.databases = cloneDatabases(tt.databases)
				op := testCloudSQLDatabaseOperation("app")
				op.DoesNotExist = tt.doesNotExist
				if tt.configure != nil {
					tt.configure(&op)
				}
				ctx := blackstart.OpContext(context.Background(), &op)

				err := (&database{runtime: api.runtime(nil)}).Set(ctx)
				require.NoError(t, err)
				require.ElementsMatch(t, tt.wantDatabases, api.databases)
				require.Equal(t, tt.wantInsert, api.requestCount(http.MethodPost, "/databases"))
				require.Equal(t, tt.wantDelete, api.requestCount(http.MethodDelete, "/databases/app"))
				if tt.assertRequest != nil {
					tt.assertRequest(t, api)
				}
			},
		)
	}
}

// TestDatabaseAPIFailures verifies Admin API failures are returned by the database module.
func TestDatabaseAPIFailures(t *testing.T) {
	tests := map[string]struct {
		method     string
		pathSuffix string
		call       func(*database, blackstart.ModuleContext) error
	}{
		"instance get": {
			method:     http.MethodGet,
			pathSuffix: "/instances/instance",
			call: func(module *database, ctx blackstart.ModuleContext) error {
				_, err := module.Check(ctx)
				return err
			},
		},
		"database get": {
			method:     http.MethodGet,
			pathSuffix: "/instances/instance/databases/app",
			call: func(module *database, ctx blackstart.ModuleContext) error {
				_, err := module.Check(ctx)
				return err
			},
		},
		"database insert": {
			method:     http.MethodPost,
			pathSuffix: "/instances/instance/databases",
			call:       func(module *database, ctx blackstart.ModuleContext) error { return module.Set(ctx) },
		},
		"database delete": {
			method:     http.MethodDelete,
			pathSuffix: "/instances/instance/databases/app",
			call: func(module *database, ctx blackstart.ModuleContext) error {
				return module.Set(ctx)
			},
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, "POSTGRES_17")
				api.databases = []*sqladmin.Database{{Name: "app", Instance: "instance", Project: "project"}}
				api.fail[tt.method+" /v1/projects/project"+tt.pathSuffix] = http.StatusInternalServerError
				op := testCloudSQLDatabaseOperation("app")
				if name == "database delete" {
					op.DoesNotExist = true
				}
				ctx := blackstart.OpContext(context.Background(), &op)

				require.Error(t, tt.call(&database{runtime: api.runtime(nil)}, ctx))
			},
		)
	}
}

// TestDatabaseSetupFailures verifies engine and option validation failures.
func TestDatabaseSetupFailures(t *testing.T) {
	tests := map[string]struct {
		version   string
		configure func(*blackstart.Operation)
		wantErr   string
	}{
		"sql server unsupported": {
			version: "SQLSERVER_2022_STANDARD",
			wantErr: `the Cloud SQL engine "SQLSERVER" is not supported by google_cloudsql_database`,
		},
		"postgres rejects charset": {
			version: "POSTGRES_17",
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputCharset] = blackstart.NewInputFromValue("utf8mb4")
			},
			wantErr: "charset and collation are only supported for MySQL Cloud SQL databases",
		},
		"postgres rejects collation": {
			version: "POSTGRES_17",
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputCollation] = blackstart.NewInputFromValue("utf8mb4_0900_ai_ci")
			},
			wantErr: "charset and collation are only supported for MySQL Cloud SQL databases",
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, tt.version)
				op := testCloudSQLDatabaseOperation("app")
				if tt.configure != nil {
					tt.configure(&op)
				}
				ctx := blackstart.OpContext(context.Background(), &op)

				_, err := (&database{runtime: api.runtime(nil)}).Check(ctx)
				require.ErrorContains(t, err, tt.wantErr)
			},
		)
	}
}

// TestDatabaseOutputErrors verifies duplicate output errors are returned.
func TestDatabaseOutputErrors(t *testing.T) {
	tests := map[string]struct {
		call func(*database, blackstart.ModuleContext) error
	}{
		"check": {
			call: func(module *database, ctx blackstart.ModuleContext) error {
				_, err := module.Check(ctx)
				return err
			},
		},
		"set": {
			call: func(module *database, ctx blackstart.ModuleContext) error {
				return module.Set(ctx)
			},
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, "POSTGRES_17")
				api.databases = []*sqladmin.Database{{Name: "app", Instance: "instance", Project: "project"}}
				op := testCloudSQLDatabaseOperation("app")
				ctx := blackstart.OpContext(context.Background(), &op)
				require.NoError(t, ctx.Output(outputDatabase, "existing"))

				err := tt.call(&database{runtime: api.runtime(nil)}, ctx)
				require.EqualError(t, err, "output key already exists: database")
			},
		)
	}
}

// TestPostgresDatabase tests database lifecycle operations against a live PostgreSQL instance.
func TestPostgresDatabase(t *testing.T) {
	testConfig := map[string]string{}
	for _, key := range []string{inputProject, inputInstance} {
		testConfig[key] = util.GetTestEnvRequiredVar(t, postgresLiveModulePackage, key)
	}
	for _, key := range []string{inputRegion, inputDatabase} {
		if value := util.GetTestEnvOptionalVar(t, postgresLiveModulePackage, key); value != "" {
			testConfig[key] = value
		}
	}
	runLiveDatabaseLifecycle(t, postgresLiveModulePackage, testConfig, nil)
}

// TestMySQLDatabase tests database lifecycle operations against a live MySQL instance.
func TestMySQLDatabase(t *testing.T) {
	testConfig := map[string]string{}
	for _, key := range []string{inputProject, inputInstance} {
		testConfig[key] = util.GetTestEnvRequiredVar(t, mysqlLiveModulePackage, key)
	}
	for _, key := range []string{inputRegion, inputDatabase, inputCharset, inputCollation} {
		if value := util.GetTestEnvOptionalVar(t, mysqlLiveModulePackage, key); value != "" {
			testConfig[key] = value
		}
	}
	runLiveDatabaseLifecycle(
		t,
		mysqlLiveModulePackage,
		testConfig,
		func(inputs map[string]blackstart.Input) {
			if testConfig[inputCharset] != "" {
				inputs[inputCharset] = blackstart.NewInputFromValue(testConfig[inputCharset])
			}
			if testConfig[inputCollation] != "" {
				inputs[inputCollation] = blackstart.NewInputFromValue(testConfig[inputCollation])
			}
		},
	)
}

// runLiveDatabaseLifecycle exercises a live Cloud SQL database create/check/delete lifecycle.
func runLiveDatabaseLifecycle(
	t *testing.T,
	modulePackage string,
	testConfig map[string]string,
	configure func(map[string]blackstart.Input),
) {
	t.Helper()
	databaseName := testConfig[inputDatabase]
	if databaseName == "" {
		databaseName = liveDatabaseName(modulePackage)
	}

	inputs := map[string]blackstart.Input{
		inputInstance: blackstart.NewInputFromValue(testConfig[inputInstance]),
		inputProject:  blackstart.NewInputFromValue(testConfig[inputProject]),
		inputRegion:   blackstart.NewInputFromValue(testConfig[inputRegion]),
		inputDatabase: blackstart.NewInputFromValue(databaseName),
	}
	if configure != nil {
		configure(inputs)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	op := blackstart.Operation{
		Inputs: inputs,
		Id:     "test-cloudsql-database",
		Name:   "test-cloudsql-database",
		Module: "google_cloudsql_database",
	}

	module := NewCloudSqlDatabase()
	t.Logf("checking Cloud SQL database %q", databaseName)
	exists, err := module.Check(blackstart.OpContext(ctx, &op))
	require.NoError(t, err)
	if exists && testConfig[inputDatabase] != "" {
		t.Logf("database %q already exists; leaving it unchanged", databaseName)
		return
	}
	require.False(t, exists)

	t.Logf("creating Cloud SQL database %q", databaseName)
	require.NoError(t, module.Set(blackstart.OpContext(ctx, &op)))
	t.Cleanup(
		func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
			defer cleanupCancel()
			op.DoesNotExist = true
			t.Logf("cleaning up Cloud SQL database %q", databaseName)
			_ = module.Set(blackstart.OpContext(cleanupCtx, &op))
		},
	)

	t.Logf("checking Cloud SQL database %q exists", databaseName)
	exists, err = module.Check(blackstart.OpContext(ctx, &op))
	require.NoError(t, err)
	require.True(t, exists)

	op.DoesNotExist = true
	t.Logf("deleting Cloud SQL database %q", databaseName)
	require.NoError(t, module.Set(blackstart.OpContext(ctx, &op)))
	t.Logf("checking Cloud SQL database %q is absent", databaseName)
	exists, err = module.Check(blackstart.OpContext(ctx, &op))
	require.NoError(t, err)
	require.True(t, exists)
}

// liveDatabaseName returns a unique live-test database name.
func liveDatabaseName(modulePackage string) string {
	prefix := strings.ReplaceAll(modulePackage, ".", "_")
	return fmt.Sprintf("blackstart_%s_%d", prefix, time.Now().UnixNano())
}

// TestDatabaseDeleteFailureIsNotCollision verifies delete API failures are surfaced directly.
func TestDatabaseDeleteFailureIsNotCollision(t *testing.T) {
	api := newFakeCloudSQLAdmin(t, "POSTGRES_17")
	api.databases = []*sqladmin.Database{{Name: "app", Instance: "instance", Project: "project"}}
	api.fail["DELETE /v1/projects/project/instances/instance/databases/app"] = http.StatusInternalServerError
	op := testCloudSQLDatabaseOperation("app")
	op.DoesNotExist = true
	ctx := blackstart.OpContext(context.Background(), &op)

	err := (&database{runtime: api.runtime(nil)}).Set(ctx)
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrMySQLUserCollision))
}
