package cloudsql

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"cloud.google.com/go/cloudsqlconn/postgres/pgxv5"
	"github.com/DATA-DOG/go-sqlmock"
	gomysql "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/sqladmin/v1"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/modules/google/cloud"
	"github.com/pezops/blackstart/util"
)

// TestPostgresManagedInstance tests managed-instance reconciliation against a live PostgreSQL instance.
func TestPostgresManagedInstance(t *testing.T) {
	var err error

	// This is a live test, the cloud config pulls settings from the environment.
	cloudConfig := map[string]string{}

	envRequiredConfig := []string{inputProject, inputInstance}
	envOptionalConfig := []string{inputRegion}

	for _, v := range envRequiredConfig {
		cloudConfig[v] = util.GetTestEnvRequiredVar(t, modulePackage, v)
	}

	for _, v := range envOptionalConfig {
		r := util.GetTestEnvOptionalVar(t, modulePackage, v)
		if r != "" {
			cloudConfig[v] = r
		}
	}

	op := blackstart.Operation{
		Inputs: map[string]blackstart.Input{
			inputInstance:       blackstart.NewInputFromValue(cloudConfig[inputInstance]),
			inputProject:        blackstart.NewInputFromValue(cloudConfig[inputProject]),
			inputConnectionType: blackstart.NewInputFromValue("PUBLIC_IP"),
		},
		Id:     "test",
		Name:   "test",
		Module: "google_cloudsql_managed_instance",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	mi := NewCloudSqlManagedInstance()
	mctx := blackstart.OpContext(ctx, &op)

	t.Log("checking the instance to see if its already managed")
	res, _ := mi.Check(mctx)
	//assert.NoError(t, err)
	assert.NotNil(t, res)
	assert.Equal(t, false, res)

	t.Log("setting instance to managed")
	err = mi.Set(mctx)
	assert.NoError(t, err)

	t.Log("checking instance to see if it's managed")
	// Checking after set will return an error about a duplicate output key
	res, _ = mi.Check(mctx)
	require.Equal(t, true, res)

	// Now ensure the instance is unmanaged
	cancel()
	ctx, cancel = context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	op.DoesNotExist = true
	mctx = blackstart.OpContext(ctx, &op)

	err = mi.Validate(op)
	require.NoError(t, err)

	t.Log("checking instance to see if it's not managed")
	res, err = mi.Check(mctx)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, false, res)

	t.Log("setting instance to not managed")
	err = mi.Set(mctx)
	require.NoError(t, err)

	t.Log("checking instance to see if it's not managed")
	res, err = mi.Check(mctx)
	require.NoError(t, err)
	require.NotNil(t, res)
	require.Equal(t, true, res)

}

// TestMySQLManagedInstance tests managed-instance reconciliation against a live MySQL instance.
func TestMySQLManagedInstance(t *testing.T) {
	cloudConfig := map[string]string{}
	for _, key := range []string{inputProject, inputInstance} {
		cloudConfig[key] = util.GetTestEnvRequiredVar(t, mysqlLiveModulePackage, key)
	}
	for _, key := range []string{inputRegion, inputUser, inputConnectionType} {
		if value := util.GetTestEnvOptionalVar(t, mysqlLiveModulePackage, key); value != "" {
			cloudConfig[key] = value
		}
	}
	if cloudConfig[inputConnectionType] == "" {
		cloudConfig[inputConnectionType] = "PUBLIC_IP"
	}

	inputs := map[string]blackstart.Input{
		inputInstance:       blackstart.NewInputFromValue(cloudConfig[inputInstance]),
		inputProject:        blackstart.NewInputFromValue(cloudConfig[inputProject]),
		inputRegion:         blackstart.NewInputFromValue(cloudConfig[inputRegion]),
		inputConnectionType: blackstart.NewInputFromValue(cloudConfig[inputConnectionType]),
	}
	if cloudConfig[inputUser] != "" {
		inputs[inputUser] = blackstart.NewInputFromValue(cloudConfig[inputUser])
	}
	op := blackstart.Operation{
		Inputs: inputs,
		Id:     "test-mysql-managed-instance",
		Name:   "test-mysql-managed-instance",
		Module: "google_cloudsql_managed_instance",
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Hour)
	defer cancel()
	module := NewCloudSqlManagedInstance()

	t.Log("setting the MySQL instance to managed")
	require.NoError(t, module.Set(blackstart.OpContext(ctx, &op)))
	t.Cleanup(
		func() {
			cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), time.Minute)
			defer cleanupCancel()
			op.DoesNotExist = true
			_ = module.Set(blackstart.OpContext(cleanupCtx, &op))
			if closer, ok := module.(interface{ Close() error }); ok {
				_ = closer.Close()
			}
		},
	)

	checkOp := op
	checkOp.Id = "check-mysql-managed-instance"
	managed, err := module.Check(blackstart.OpContext(ctx, &checkOp))
	require.NoError(t, err)
	require.True(t, managed)

	op.DoesNotExist = true
	t.Log("setting the MySQL instance to unmanaged")
	require.NoError(t, module.Set(blackstart.OpContext(ctx, &op)))

	checkOp = op
	checkOp.Id = "check-mysql-unmanaged-instance"
	managed, err = module.Check(blackstart.OpContext(ctx, &checkOp))
	require.NoError(t, err)
	require.True(t, managed)
}

// TestPostgresConnectUser tests an IAM-authenticated connection to a live PostgreSQL instance.
func TestPostgresConnectUser(t *testing.T) {

	// This is a live test, the cloud config pulls settings from the environment.
	testConfig := map[string]string{
		inputDatabase: "postgres",
	}

	envRequiredConfig := []string{inputProject, inputInstance}
	envOptionalConfig := []string{inputDatabase}

	for _, v := range envRequiredConfig {
		testConfig[v] = util.GetTestEnvRequiredVar(t, modulePackage, v)
	}

	for _, v := range envOptionalConfig {
		r := util.GetTestEnvOptionalVar(t, modulePackage, v)
		if r != "" {
			testConfig[v] = r
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	instance := testConfig[inputInstance]
	project := testConfig[inputProject]
	dbName := testConfig[inputDatabase]
	creds, err := cloud.DefaultCredentials(ctx)
	require.NoError(t, err)
	iamIdentity, err := cloud.IamUser(ctx, creds)
	require.NoError(t, err)
	username, err := postgresAdcIamUser(ctx)
	require.NoError(t, err)
	ensureLiveCloudSQLIAMUser(
		t,
		ctx,
		project,
		testConfig[inputRegion],
		instance,
		iamIdentity,
		iamUserType(creds, iamIdentity),
	)

	connConfig := connectionConfig{
		instance: instance,
		project:  project,
		database: dbName,
	}
	instanceIdentifier, err := connConfig.connectionIdentifier(ctx)
	if err != nil {
		log.Fatalf("Error getting instance identifier: %v", err)
	}

	// Connect to the instance and run the query
	dsn := cloudsqlPostgresIamDsn(instanceIdentifier, dbName, username)

	db, err := sql.Open(sqlDriverPostgresIam, dsn)
	require.NoError(t, err)

	// Run SELECT 1
	var result int
	err = db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	require.NoError(t, err)

	t.Logf("Query result: %d", result)
}

// TestMySQLConnectUser tests an IAM-authenticated connection to a live MySQL instance.
func TestMySQLConnectUser(t *testing.T) {
	testConfig := map[string]string{}
	for _, key := range []string{inputProject, inputInstance} {
		testConfig[key] = util.GetTestEnvRequiredVar(t, mysqlLiveModulePackage, key)
	}
	for _, key := range []string{inputRegion, inputDatabase, inputUser, inputConnectionType} {
		if value := util.GetTestEnvOptionalVar(t, mysqlLiveModulePackage, key); value != "" {
			testConfig[key] = value
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	creds, err := cloud.DefaultCredentials(ctx)
	require.NoError(t, err)
	iamIdentity := testConfig[inputUser]
	if iamIdentity == "" {
		iamIdentity, err = cloud.IamUser(ctx, creds)
		require.NoError(t, err)
	}
	ensureLiveCloudSQLIAMUser(
		t,
		ctx,
		testConfig[inputProject],
		testConfig[inputRegion],
		testConfig[inputInstance],
		iamIdentity,
		iamUserType(creds, iamIdentity),
	)
	username, err := mysqlIamUser(iamIdentity)
	require.NoError(t, err)

	connConfig := connectionConfig{
		instance: testConfig[inputInstance],
		project:  testConfig[inputProject],
		region:   testConfig[inputRegion],
		creds:    creds,
	}
	instanceIdentifier, err := connConfig.connectionIdentifier(ctx)
	require.NoError(t, err)

	driver := sqlDriverMySQLIam
	if strings.EqualFold(testConfig[inputConnectionType], "PRIVATE_IP") {
		driver = sqlDriverMySQLIamPrivateIp
	}
	db, err := sql.Open(
		driver,
		cloudsqlMySQLDsn(driver, instanceIdentifier, testConfig[inputDatabase], username, ""),
	)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, db.Close())
	}()

	var result int
	require.NoError(t, db.QueryRowContext(ctx, "SELECT 1").Scan(&result))
	require.Equal(t, 1, result)
}

// TestPostgresConnectSvcAcct tests a service-account connection to a live PostgreSQL instance.
func TestPostgresConnectSvcAcct(t *testing.T) {
	// This is a live test, the cloud config pulls settings from the environment.
	testConfig := map[string]string{
		inputDatabase: "postgres",
	}

	envRequiredConfig := []string{inputProject, inputInstance}
	envOptionalConfig := []string{inputDatabase, "svc_acct_json"}

	for _, v := range envRequiredConfig {
		testConfig[v] = util.GetTestEnvRequiredVar(t, modulePackage, v)
	}

	for _, v := range envOptionalConfig {
		r := util.GetTestEnvOptionalVar(t, modulePackage, v)
		if r != "" {
			testConfig[v] = r
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	instance := testConfig[inputInstance]
	project := testConfig[inputProject]
	dbName := testConfig[inputDatabase]
	filename := testConfig["svc_acct_json"]

	// Service account JSON key file
	var creds *google.Credentials
	if filename != "" {
		// Load CredentialsFromJSON from file
		b, err := os.ReadFile(filename)
		if err != nil {
			t.Fatalf("Error reading credentials file: %v", err)
		}
		creds, err = google.CredentialsFromJSONWithTypeAndParams(
			ctx,
			b,
			google.ServiceAccount,
			google.CredentialsParams{Scopes: []string{"https://www.googleapis.com/auth/cloud-platform"}},
		)
		if err != nil {
			t.Fatalf("Error creating credentials: %v", err)
		}
	} else {
		var err error
		creds, err = cloud.DefaultCredentials(ctx)
		if err != nil {
			t.Fatalf("Error obtaining credentials: %v", err)
		}
	}

	connConfig := connectionConfig{
		instance: instance,
		project:  project,
		database: dbName,
		creds:    creds,
	}
	instanceIdentifier, err := connConfig.connectionIdentifier(ctx)
	if err != nil {
		log.Fatalf("Error getting instance identifier: %v", err)
	}

	userId, err := postgresIamUser(ctx, creds)
	if err != nil {
		log.Fatalf("Error finding IAM user: %v", err)
	}
	ensureLiveCloudSQLIAMUser(
		t,
		ctx,
		project,
		testConfig[inputRegion],
		instance,
		userId,
		iamUserType(creds, userId),
	)

	dsn := cloudsqlPostgresIamDsn(instanceIdentifier, dbName, userId)

	var db *sql.DB
	if filename != "" {
		_, _ = pgxv5.RegisterDriver(
			sqlDriverPostgresIam+"svc_acct", cloudsqlconn.WithIAMAuthN(), cloudsqlconn.WithCredentialsFile(filename),
		)

		db, err = sql.Open(sqlDriverPostgresIam+"svc_acct", dsn)
		if err != nil {
			t.Errorf("failed to open database connection: %v", err)
		}
	} else {
		db, err = sql.Open(sqlDriverPostgresIam, dsn)
		if err != nil {
			t.Errorf("failed to open database connection: %v", err)
		}
	}

	// Run SELECT 1
	var result int
	err = db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	if err != nil {
		t.Errorf("failed to run query: %v", err)
	}

	require.Equal(t, 1, result)
}

// ensureLiveCloudSQLIAMUser creates the IAM database user needed by live connection tests.
func ensureLiveCloudSQLIAMUser(
	t *testing.T,
	ctx context.Context,
	project string,
	region string,
	instance string,
	iamIdentity string,
	userType string,
) {
	t.Helper()
	op := blackstart.Operation{
		Inputs: map[string]blackstart.Input{
			inputInstance: blackstart.NewInputFromValue(instance),
			inputProject:  blackstart.NewInputFromValue(project),
			inputRegion:   blackstart.NewInputFromValue(region),
			inputUser:     blackstart.NewInputFromValue(iamIdentity),
			inputUserType: blackstart.NewInputFromValue(userType),
		},
		Id:     "ensure-live-iam-user",
		Name:   "ensure-live-iam-user",
		Module: "google_cloudsql_user",
	}

	module := NewCloudSqlUser()
	exists, err := module.Check(blackstart.OpContext(ctx, &op))
	require.NoError(t, err)
	if exists {
		return
	}

	require.NoError(t, module.Set(blackstart.OpContext(ctx, &op)))
	t.Cleanup(
		func() {
			cleanupCtx, cancel := context.WithTimeout(context.Background(), time.Minute)
			defer cancel()
			op.DoesNotExist = true
			_ = module.Set(blackstart.OpContext(cleanupCtx, &op))
		},
	)
}

func TestIsManagedInstanceBootstrapConnectionError(t *testing.T) {
	tests := map[string]struct {
		err    error
		engine string
		want   bool
	}{
		"mysql access denied": {
			err:    &gomysql.MySQLError{Number: 1045, Message: "access denied"},
			engine: "MYSQL",
			want:   true,
		},
		"mysql unknown database": {
			err:    &gomysql.MySQLError{Number: 1049, Message: "unknown database"},
			engine: "MYSQL",
		},
		"postgres unrelated error": {
			err:    assert.AnError,
			engine: "POSTGRES",
		},
		"postgres invalid password": {
			err:    &testError{message: "connection failed: SQLSTATE 28P01"},
			engine: "POSTGRES",
			want:   true,
		},
	}
	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				assert.Equal(t, tt.want, isManagedInstanceBootstrapConnectionError(tt.err, tt.engine))
			},
		)
	}
}

func TestManagedInstanceMySQLDrivers(t *testing.T) {
	tests := map[string]struct {
		connectionType string
		iamDriver      string
		builtinDriver  string
	}{
		"public IP": {
			connectionType: "PUBLIC_IP",
			iamDriver:      sqlDriverMySQLIam,
			builtinDriver:  sqlDriverMySQL,
		},
		"private IP": {
			connectionType: "PRIVATE_IP",
			iamDriver:      sqlDriverMySQLIamPrivateIp,
			builtinDriver:  sqlDriverMySQLPrivateIp,
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				op := blackstart.Operation{
					Inputs: map[string]blackstart.Input{
						inputConnectionType: blackstart.NewInputFromValue(tt.connectionType),
					},
					Module: "google_cloudsql_managed_instance",
				}
				ctx := blackstart.OpContext(context.Background(), &op)
				m := managedInstance{target: &connectionConfig{engine: "MYSQL"}}

				iamDriver, err := m.getDriver(ctx)
				require.NoError(t, err)
				assert.Equal(t, tt.iamDriver, iamDriver)

				builtinDriver, err := m.getBuiltinDriver(ctx)
				require.NoError(t, err)
				assert.Equal(t, tt.builtinDriver, builtinDriver)
			},
		)
	}
}

// TestManagedInstanceValidate verifies static input validation and permits runtime inputs.
func TestManagedInstanceValidate(t *testing.T) {
	tests := map[string]struct {
		configure func(*blackstart.Operation)
		wantErr   string
	}{
		"valid": {
			configure: func(*blackstart.Operation) {},
		},
		"valid lowercase connection type": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputConnectionType] = blackstart.NewInputFromValue("public_ip")
			},
		},
		"runtime instance skips static validation": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputInstance] = blackstart.NewInputFromDep("create-instance", "name")
			},
		},
		"missing instance": {
			configure: func(op *blackstart.Operation) {
				delete(op.Inputs, inputInstance)
			},
			wantErr: "missing required parameter: instance",
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
		"invalid connection type value": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputConnectionType] = blackstart.NewInputFromValue("DIRECT")
			},
			wantErr: "invalid connection_type: DIRECT - must be one of: PUBLIC_IP, PRIVATE_IP",
		},
		"invalid connection type type": {
			configure: func(op *blackstart.Operation) {
				op.Inputs[inputConnectionType] = blackstart.NewInputFromValue(map[string]string{"type": "PUBLIC_IP"})
			},
			wantErr: "invalid connection_type:",
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				op := testManagedInstanceOperation("person@example.com")
				tt.configure(&op)

				err := (&managedInstance{}).Validate(op)
				if tt.wantErr == "" {
					require.NoError(t, err)
					return
				}
				require.ErrorContains(t, err, tt.wantErr)
			},
		)
	}
}

// TestManagedInstanceCheckWithMocks verifies managed-role checks for PostgreSQL and MySQL.
func TestManagedInstanceCheckWithMocks(t *testing.T) {
	tests := map[string]struct {
		version      string
		userName     string
		doesNotExist bool
		roleResult   int
		noRoleRow    bool
		want         bool
	}{
		"postgres managed": {
			version:    "POSTGRES_17",
			userName:   "person@example.com",
			roleResult: 1,
			want:       true,
		},
		"postgres unmanaged": {
			version:   "POSTGRES_17",
			userName:  "person@example.com",
			noRoleRow: true,
		},
		"mysql managed": {
			version:    "MYSQL_8_4",
			userName:   "person@example.com",
			roleResult: 1,
			want:       true,
		},
		"mysql unmanaged satisfies does not exist": {
			version:      "MYSQL_8_4",
			userName:     "person@example.com",
			doesNotExist: true,
			want:         true,
		},
	}

	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, tt.version)
				opener := newQueuedDBOpener(t)
				driver, dsn, roleQuery := expectedManagedConnection(tt.version, tt.userName)
				_, mock := opener.expect(driver, dsn)
				mock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow(1))
				roleExpectation := mock.ExpectQuery(regexp.QuoteMeta(roleQuery))
				if tt.noRoleRow {
					roleExpectation.WillReturnError(sql.ErrNoRows)
				} else {
					roleExpectation.WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(tt.roleResult))
				}
				mock.ExpectClose()

				op := testManagedInstanceOperation(tt.userName)
				op.DoesNotExist = tt.doesNotExist
				ctx := blackstart.OpContext(context.Background(), &op)
				module := &managedInstance{
					creds:   &google.Credentials{ProjectID: "project"},
					runtime: api.runtime(opener.open),
				}
				got, err := module.Check(ctx)
				require.NoError(t, err)
				require.Equal(t, tt.want, got)
				require.NoError(t, module.Close())
				require.NoError(t, mock.ExpectationsWereMet())
				opener.verify()
			},
		)
	}
}

// TestManagedInstanceCheckBootstrapAuthenticationFailure verifies bootstrap authentication misses.
func TestManagedInstanceCheckBootstrapAuthenticationFailure(t *testing.T) {
	for _, doesNotExist := range []bool{false, true} {
		t.Run(
			"doesNotExist="+strconv.FormatBool(doesNotExist), func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, "MYSQL_8_4")
				opener := newQueuedDBOpener(t)
				driver, dsn, _ := expectedManagedConnection("MYSQL_8_4", "person@example.com")
				_, mock := opener.expect(driver, dsn)
				mock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).WillReturnError(&gomysql.MySQLError{Number: 1045})

				op := testManagedInstanceOperation("person@example.com")
				op.DoesNotExist = doesNotExist
				ctx := blackstart.OpContext(context.Background(), &op)
				module := &managedInstance{
					creds:   &google.Credentials{ProjectID: "project"},
					runtime: api.runtime(opener.open),
				}
				got, err := module.Check(ctx)
				require.NoError(t, err)
				require.Equal(t, doesNotExist, got)
				require.NoError(t, mock.ExpectationsWereMet())
			},
		)
	}
}

// TestManagedInstanceCheckErrors verifies errors returned after a successful setup.
func TestManagedInstanceCheckErrors(t *testing.T) {
	t.Run(
		"role query failure", func(t *testing.T) {
			api := newFakeCloudSQLAdmin(t, "POSTGRES_17")
			opener := newQueuedDBOpener(t)
			driver, dsn, roleQuery := expectedManagedConnection("POSTGRES_17", "person@example.com")
			_, mock := opener.expect(driver, dsn)
			mock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).
				WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow(1))
			mock.ExpectQuery(regexp.QuoteMeta(roleQuery)).WillReturnError(errors.New("role query failed"))
			mock.ExpectClose()

			op := testManagedInstanceOperation("person@example.com")
			ctx := blackstart.OpContext(context.Background(), &op)
			module := &managedInstance{
				creds:   &google.Credentials{ProjectID: "project"},
				runtime: api.runtime(opener.open),
			}
			_, err := module.Check(ctx)
			require.ErrorContains(
				t, err,
				"failed to check if user is a member of cloudsqlsuperuser role: "+
					"failed to check if user is a member of cloudsqlsuperuser role: role query failed",
			)
			require.NoError(t, mock.ExpectationsWereMet())
		},
	)

	t.Run(
		"duplicate connection output", func(t *testing.T) {
			api := newFakeCloudSQLAdmin(t, "POSTGRES_17")
			opener := newQueuedDBOpener(t)
			driver, dsn, roleQuery := expectedManagedConnection("POSTGRES_17", "person@example.com")
			_, mock := opener.expect(driver, dsn)
			mock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).
				WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow(1))
			mock.ExpectQuery(regexp.QuoteMeta(roleQuery)).
				WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(1))
			mock.ExpectClose()

			op := testManagedInstanceOperation("person@example.com")
			ctx := blackstart.OpContext(context.Background(), &op)
			require.NoError(t, ctx.Output(outputConnection, "existing"))
			module := &managedInstance{
				creds:   &google.Credentials{ProjectID: "project"},
				runtime: api.runtime(opener.open),
			}
			got, err := module.Check(ctx)
			require.True(t, got)
			require.EqualError(t, err, "output key already exists: connection")
			require.NoError(t, mock.ExpectationsWereMet())
		},
	)
}

// TestManagedInstanceSetMySQLWithMocks verifies the complete MySQL management workflow.
func TestManagedInstanceSetMySQLWithMocks(t *testing.T) {
	api := newFakeCloudSQLAdmin(t, "MYSQL_8_4")
	opener := newQueuedDBOpener(t)
	_, tempMock := opener.expectContaining(
		sqlDriverMySQL,
		"blackstart:",
		"@cloudsql-mysql(project:us-central1:instance)/mysql",
	)
	tempMock.ExpectExec(regexp.QuoteMeta("GRANT `cloudsqlsuperuser` TO `person`@`%` WITH ADMIN OPTION")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	tempMock.ExpectExec(regexp.QuoteMeta("SET DEFAULT ROLE ALL TO `person`@`%`")).
		WillReturnResult(sqlmock.NewResult(0, 0))
	tempMock.ExpectClose()

	driver, dsn, _ := expectedManagedConnection("MYSQL_8_4", "person@example.com")
	_, managedMock := opener.expect(driver, dsn)
	managedMock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow(1))
	managedMock.ExpectClose()

	op := testManagedInstanceOperation("person@example.com")
	ctx := blackstart.OpContext(context.Background(), &op)
	module := &managedInstance{
		creds:   &google.Credentials{ProjectID: "project"},
		runtime: api.runtime(opener.open),
	}
	require.NoError(t, module.Set(ctx))
	require.NoError(t, module.Close())
	require.NoError(t, tempMock.ExpectationsWereMet())
	require.NoError(t, managedMock.ExpectationsWereMet())
	opener.verify()

	require.Equal(t, 2, api.requestCount("POST", "/users"))
	require.Equal(t, 1, api.requestCount("DELETE", "/users"))
	require.Len(t, api.users, 1)
	require.Equal(t, "person", api.users[0].Name)
	require.Equal(t, "person@example.com", api.users[0].IamEmail)
}

// TestManagedInstanceSetDoesNotExistRevokesWithoutDeletingIAMUser verifies unmanagement behavior.
func TestManagedInstanceSetDoesNotExistRevokesWithoutDeletingIAMUser(t *testing.T) {
	api := newFakeCloudSQLAdmin(t, "POSTGRES_17")
	api.users = []*sqladmin.User{{Name: "person@example.com", Host: "%", Type: userCloudIamUser}}
	opener := newQueuedDBOpener(t)
	_, tempMock := opener.expectContaining(
		sqlDriverPostgres,
		"user=blackstart",
		"dbname=postgres",
	)
	tempMock.ExpectExec(regexp.QuoteMeta(`REVOKE cloudsqlsuperuser FROM "person@example.com";`)).
		WillReturnResult(sqlmock.NewResult(0, 0))
	tempMock.ExpectClose()

	driver, dsn, _ := expectedManagedConnection("POSTGRES_17", "person@example.com")
	_, managedMock := opener.expect(driver, dsn)
	managedMock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).WillReturnRows(sqlmock.NewRows([]string{"result"}).AddRow(1))
	managedMock.ExpectClose()

	op := testManagedInstanceOperation("person@example.com")
	op.DoesNotExist = true
	ctx := blackstart.OpContext(context.Background(), &op)
	module := &managedInstance{
		creds:   &google.Credentials{ProjectID: "project"},
		runtime: api.runtime(opener.open),
	}
	require.NoError(t, module.Set(ctx))
	require.NoError(t, module.Close())
	require.NoError(t, tempMock.ExpectationsWereMet())
	require.NoError(t, managedMock.ExpectationsWereMet())
	require.Len(t, api.users, 1)
	require.Equal(t, "person@example.com", api.users[0].Name)
}

// TestManagedInstanceSetFailureStillCleansUpTemporaryUser verifies temporary-user cleanup on failure.
func TestManagedInstanceSetFailureStillCleansUpTemporaryUser(t *testing.T) {
	api := newFakeCloudSQLAdmin(t, "MYSQL_8_4")
	opener := newQueuedDBOpener(t)
	_, tempMock := opener.expectContaining(
		sqlDriverMySQL,
		"blackstart:",
		"@cloudsql-mysql(project:us-central1:instance)/mysql",
	)
	tempMock.ExpectExec(regexp.QuoteMeta("GRANT `cloudsqlsuperuser` TO `person`@`%` WITH ADMIN OPTION")).
		WillReturnError(errors.New("grant failed"))
	tempMock.ExpectClose()

	op := testManagedInstanceOperation("person@example.com")
	ctx := blackstart.OpContext(context.Background(), &op)
	module := &managedInstance{
		creds:   &google.Credentials{ProjectID: "project"},
		runtime: api.runtime(opener.open),
	}
	require.EqualError(t, module.Set(ctx), "failed to grant MySQL cloudsqlsuperuser role: grant failed")
	require.NoError(t, tempMock.ExpectationsWereMet())
	opener.verify()

	require.Equal(t, 2, api.requestCount("POST", "/users"))
	require.Equal(t, 1, api.requestCount("DELETE", "/users"))
	require.Len(t, api.users, 1)
	require.Equal(t, "person", api.users[0].Name)
}

// TestManagedInstanceSetupFailures verifies instance metadata and IAM validation failures.
func TestManagedInstanceSetupFailures(t *testing.T) {
	tests := map[string]struct {
		version        string
		disableIAM     bool
		instanceStatus int
		wantErr        string
	}{
		"unsupported mysql": {
			version: "MYSQL_5_7",
			wantErr: "google_cloudsql_managed_instance supports MySQL 8+; instance uses MYSQL_5_7",
		},
		"unsupported sql server": {
			version: "SQLSERVER_2022_STANDARD",
			wantErr: `the Cloud SQL engine "SQLSERVER" is not supported by google_cloudsql_managed_instance`,
		},
		"iam disabled": {
			version:    "POSTGRES_17",
			disableIAM: true,
			wantErr:    "instance instance in project project does not have IAM database authentication enabled",
		},
		"instance API failure": {
			version:        "POSTGRES_17",
			instanceStatus: 500,
			wantErr:        "failed to get instance instance in project project:",
		},
		"instance does not exist": {
			version:        "POSTGRES_17",
			instanceStatus: 404,
			wantErr:        "instance instance does not exist in project project",
		},
	}
	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, tt.version)
				if tt.disableIAM {
					api.instance.Settings.DatabaseFlags[0].Value = "off"
				}
				if tt.instanceStatus != 0 {
					api.fail["GET /v1/projects/project/instances/instance"] = tt.instanceStatus
				}
				op := testManagedInstanceOperation("person@example.com")
				ctx := blackstart.OpContext(context.Background(), &op)
				module := &managedInstance{
					creds:   &google.Credentials{ProjectID: "project"},
					runtime: api.runtime(nil),
				}
				_, err := module.Check(ctx)
				require.ErrorContains(t, err, tt.wantErr)
			},
		)
	}
}

// TestSetManagedRoleErrors verifies role-management SQL failures are returned.
func TestSetManagedRoleErrors(t *testing.T) {
	tests := map[string]struct {
		engine  string
		query   string
		wantErr string
	}{
		"postgres grant": {
			engine:  "POSTGRES",
			query:   `GRANT cloudsqlsuperuser TO "person@example.com" WITH ADMIN OPTION;`,
			wantErr: "failed to grant cloudsqlsuperuser role: grant failed",
		},
		"mysql grant": {
			engine:  "MYSQL",
			query:   "GRANT `cloudsqlsuperuser` TO `person`@`%` WITH ADMIN OPTION",
			wantErr: "failed to grant MySQL cloudsqlsuperuser role: grant failed",
		},
	}
	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectExec(regexp.QuoteMeta(tt.query)).WillReturnError(errors.New("grant failed"))
				op := testManagedInstanceOperation("person@example.com")
				ctx := blackstart.OpContext(context.Background(), &op)
				module := managedInstance{target: &connectionConfig{engine: tt.engine}}
				require.EqualError(t, module.setManagedRole(ctx, db, "person@example.com"), tt.wantErr)
				require.NoError(t, mock.ExpectationsWereMet())
			},
		)
	}
}

// TestSetManagedRoleSecondaryErrors verifies errors after initial role-management statements.
func TestSetManagedRoleSecondaryErrors(t *testing.T) {
	tests := map[string]struct {
		engine      string
		iamIdentity string
		firstQuery  string
		failedQuery string
		wantErr     string
	}{
		"postgres alter": {
			engine:      "POSTGRES",
			iamIdentity: "person@example.com",
			firstQuery:  `GRANT cloudsqlsuperuser TO "person@example.com" WITH ADMIN OPTION;`,
			failedQuery: `ALTER ROLE "person@example.com" WITH INHERIT CREATEROLE CREATEDB;`,
			wantErr:     "failed to update management role: update failed",
		},
		"mysql default role": {
			engine:      "MYSQL",
			iamIdentity: "person@example.com",
			firstQuery:  "GRANT `cloudsqlsuperuser` TO `person`@`%` WITH ADMIN OPTION",
			failedQuery: "SET DEFAULT ROLE ALL TO `person`@`%`",
			wantErr:     "failed to set default MySQL cloudsqlsuperuser role: update failed",
		},
	}
	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				mock.ExpectExec(regexp.QuoteMeta(tt.firstQuery)).WillReturnResult(sqlmock.NewResult(0, 0))
				mock.ExpectExec(regexp.QuoteMeta(tt.failedQuery)).WillReturnError(errors.New("update failed"))

				op := testManagedInstanceOperation(tt.iamIdentity)
				ctx := blackstart.OpContext(context.Background(), &op)
				module := managedInstance{target: &connectionConfig{engine: tt.engine}}
				require.ErrorContains(t, module.setManagedRole(ctx, db, tt.iamIdentity), tt.wantErr)
				require.NoError(t, mock.ExpectationsWereMet())
			},
		)
	}

	t.Run(
		"invalid mysql identity", func(t *testing.T) {
			db, _, err := sqlmock.New()
			require.NoError(t, err)
			op := testManagedInstanceOperation("not-an-email")
			ctx := blackstart.OpContext(context.Background(), &op)
			module := managedInstance{target: &connectionConfig{engine: "MYSQL"}}
			err = module.setManagedRole(ctx, db, "not-an-email")
			require.EqualError(t, err, "MySQL IAM identity must be an email address: not-an-email")
		},
	)

	t.Run(
		"unsupported engine", func(t *testing.T) {
			db, _, err := sqlmock.New()
			require.NoError(t, err)
			op := testManagedInstanceOperation("person@example.com")
			ctx := blackstart.OpContext(context.Background(), &op)
			module := managedInstance{target: &connectionConfig{engine: "SQLSERVER"}}
			err = module.setManagedRole(ctx, db, "person@example.com")
			require.EqualError(t, err, "unsupported Cloud SQL engine for role management: SQLSERVER")
		},
	)
}

// TestSetManagedRoleSuccess verifies engine-specific grant and revoke statements.
func TestSetManagedRoleSuccess(t *testing.T) {
	tests := map[string]struct {
		engine       string
		doesNotExist bool
		queries      []string
	}{
		"postgres grant": {
			engine: "POSTGRES",
			queries: []string{
				`GRANT cloudsqlsuperuser TO "person@example.com" WITH ADMIN OPTION;`,
				`ALTER ROLE "person@example.com" WITH INHERIT CREATEROLE CREATEDB;`,
			},
		},
		"postgres revoke": {
			engine:       "POSTGRES",
			doesNotExist: true,
			queries:      []string{`REVOKE cloudsqlsuperuser FROM "person@example.com";`},
		},
		"mysql grant": {
			engine: "MYSQL",
			queries: []string{
				"GRANT `cloudsqlsuperuser` TO `person`@`%` WITH ADMIN OPTION",
				"SET DEFAULT ROLE ALL TO `person`@`%`",
			},
		},
		"mysql revoke": {
			engine:       "MYSQL",
			doesNotExist: true,
			queries:      []string{"REVOKE `cloudsqlsuperuser` FROM `person`@`%`"},
		},
	}
	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				db, mock, err := sqlmock.New()
				require.NoError(t, err)
				for _, query := range tt.queries {
					mock.ExpectExec(regexp.QuoteMeta(query)).WillReturnResult(sqlmock.NewResult(0, 0))
				}
				op := testManagedInstanceOperation("person@example.com")
				op.DoesNotExist = tt.doesNotExist
				ctx := blackstart.OpContext(context.Background(), &op)
				module := managedInstance{target: &connectionConfig{engine: tt.engine}}
				require.NoError(t, module.setManagedRole(ctx, db, "person@example.com"))
				require.NoError(t, mock.ExpectationsWereMet())
			},
		)
	}
}

// TestManagedInstanceDatabaseFailures verifies database open and validation-query failures.
func TestManagedInstanceDatabaseFailures(t *testing.T) {
	tests := map[string]struct {
		openErr  error
		queryErr error
		wantErr  string
	}{
		"open failure": {
			openErr: errors.New("open failed"),
			wantErr: "failed to open database connection: failed to open database connection: open failed",
		},
		"query failure": {
			queryErr: errors.New("query failed"),
			wantErr:  "failed to open database connection: failed to run query: query failed",
		},
	}
	for name, tt := range tests {
		t.Run(
			name, func(t *testing.T) {
				api := newFakeCloudSQLAdmin(t, "POSTGRES_17")
				var opener func(string, string) (*sql.DB, error)
				if tt.openErr != nil {
					opener = func(string, string) (*sql.DB, error) { return nil, tt.openErr }
				} else {
					queue := newQueuedDBOpener(t)
					driver, dsn, _ := expectedManagedConnection("POSTGRES_17", "person@example.com")
					_, mock := queue.expect(driver, dsn)
					mock.ExpectQuery(regexp.QuoteMeta("SELECT 1")).WillReturnError(tt.queryErr)
					mock.ExpectClose()
					opener = queue.open
				}
				op := testManagedInstanceOperation("person@example.com")
				ctx := blackstart.OpContext(context.Background(), &op)
				module := &managedInstance{
					creds:   &google.Credentials{ProjectID: "project"},
					runtime: api.runtime(opener),
				}
				_, err := module.Check(ctx)
				require.EqualError(t, err, tt.wantErr)
			},
		)
	}
}

// expectedManagedConnection returns the expected driver, DSN, and role query for a test instance.
func expectedManagedConnection(databaseVersion, userName string) (string, string, string) {
	identifier := "project:us-central1:instance"
	if instanceEngine(databaseVersion) == "MYSQL" {
		localName, _ := mysqlIamUser(userName)
		return sqlDriverMySQLIam,
			cloudsqlMySQLDsn(sqlDriverMySQLIam, identifier, "", localName, ""),
			checkMySQLCloudSqlSuperuserRoleQuery
	}
	return sqlDriverPostgresIam,
		cloudsqlPostgresIamDsn(identifier, "postgres", userName),
		checkPostgresCloudSqlSuperuserRoleQuery
}
