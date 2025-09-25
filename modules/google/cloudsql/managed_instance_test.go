package cloudsql

import (
	"context"
	"database/sql"
	"log"
	"os"
	"testing"
	"time"

	"cloud.google.com/go/cloudsqlconn"
	"cloud.google.com/go/cloudsqlconn/postgres/pgxv5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2/google"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/modules/google/cloud"
	"github.com/pezops/blackstart/util"
)

func TestManagedInstance(t *testing.T) {
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

func TestConnectUser(t *testing.T) {

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
	username, err := postgresAdcIamUser(ctx)
	require.NoError(t, err)

	dsn := cloudsqlPostgresIamDsn(instanceIdentifier, "postgres", username)

	db, err := sql.Open(sqlDriverPostgresIam, dsn)
	require.NoError(t, err)

	// Run SELECT 1
	var result int
	err = db.QueryRowContext(ctx, "SELECT 1").Scan(&result)
	require.NoError(t, err)

	t.Logf("Query result: %d", result)
}

func TestConnectSvcAcct(t *testing.T) {
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
		creds, err = google.CredentialsFromJSON(ctx, b, "https://www.googleapis.com/auth/cloud-platform")
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

	dsn := cloudsqlPostgresIamDsn(instanceIdentifier, "postgres", userId)

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
