package cloudsql

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/cloudsqlconn"
	"cloud.google.com/go/cloudsqlconn/postgres/pgxv5"
	"github.com/pezops/blackstart/modules/google/cloud"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sqladmin/v1"
	cloudsqlv1 "google.golang.org/genproto/googleapis/cloud/sql/v1"

	"github.com/pezops/blackstart"
)

const (
	modulePackage = "google.cloudsql"

	// sqlDriverPostgresIam is the driver name for connecting to CloudSQL Postgres instances
	// using IAM credentials.
	sqlDriverPostgresIam = "cloudsql-postgres-iam"

	// sqlDriverPostgresIamPrivateIp is the driver name for connecting to CloudSQL Postgres instances
	// using IAM credentials connecting via a VPC and private IP address.
	sqlDriverPostgresIamPrivateIp = "cloudsql-postgres-iam-private"

	// sqlDriverPostgres is the driver name for connecting to CloudSQL Postgres instances using
	// built-in / static credentials.
	sqlDriverPostgres = "cloudsql-postgres"

	// sqlDriverPostgresPrivateIp is the driver name for connecting to CloudSQL Postgres instances using
	// built-in / static credentials connecting via a VPC and private IP address.
	sqlDriverPostgresPrivateIp = "cloudsql-postgres-private"

	// userBuiltIn is the user type for built-in users with static credentials. We don't
	// normally allow this, but it is used internally to bootstrap a managed instance.
	userBuiltIn = "BUILT_IN"

	// userCloudIamUser is the user type for Cloud IAM users.
	userCloudIamUser = "CLOUD_IAM_USER"

	// userCloudIamServiceAccount is the user type for Cloud IAM service accounts.
	userCloudIamServiceAccount = "CLOUD_IAM_SERVICE_ACCOUNT"

	inputInstance       = "instance"
	inputDatabase       = "database"
	inputProject        = "project"
	inputRegion         = "region"
	inputUser           = "user"
	inputPassword       = "password"
	inputConnectionType = "connection_type"
	inputUserType       = "user_type"

	outputUser       = "user"
	outputConnection = "connection"
)

func init() {
	blackstart.RegisterPathName("cloudsql", "CloudSQL")

	// Register global drivers that can be used by the database/sql package with the new package
	// to connect to CloudSQL, cloud.google.com/go/cloudsqlconn. This has a postgres subpackage
	// that is used to connect to CloudSQL Postgres instances without the need for the CloudSQL
	// proxy.
	//
	// To leverage a credentials file, a driver per credential is needed tht specifies the file
	// or contents.
	_, _ = pgxv5.RegisterDriver(sqlDriverPostgresIam, cloudsqlconn.WithIAMAuthN())
	_, _ = pgxv5.RegisterDriver(
		sqlDriverPostgresIamPrivateIp, cloudsqlconn.WithIAMAuthN(),
		cloudsqlconn.WithDefaultDialOptions(
			cloudsqlconn.WithPrivateIP(),
		),
	)
	_, _ = pgxv5.RegisterDriver(sqlDriverPostgres)
	_, _ = pgxv5.RegisterDriver(
		sqlDriverPostgresPrivateIp,
		cloudsqlconn.WithDefaultDialOptions(
			cloudsqlconn.WithPrivateIP(),
		),
	)

	// Quick check to make sure these shorter constants here did not change from the upstream values.
	if userBuiltIn != cloudsqlv1.User_SqlUserType_name[int32(cloudsqlv1.User_BUILT_IN)] ||
		userCloudIamUser != cloudsqlv1.User_SqlUserType_name[int32(cloudsqlv1.User_CLOUD_IAM_USER)] ||
		userCloudIamServiceAccount != cloudsqlv1.User_SqlUserType_name[int32(cloudsqlv1.User_CLOUD_IAM_SERVICE_ACCOUNT)] {
		panic("cloudsql user type names do not match")
	}
}

var ErrRegionNotProvidedNotFound = errors.New("region not provided and not found")
var ErrProjectNotProvidedNotFound = errors.New("project not provided and not found")

// connectionConfig is the configuration for a connection to a CloudSQL instance. It provides some
// convenience methods for working with the CloudSQL instance.
type connectionConfig struct {
	// instance is the name of the CloudSQL instance.
	instance string

	// database is the name of the database to connect to on the CloudSQL instance.
	database string

	// project is the Google Cloud project ID. This value will attempt to be determined from the
	// credentials if not provided.
	project string

	// region is the region where the CloudSQL instance is located. If not provided, the region
	// will attempt to be determined using the sqladmin API.
	region string

	// user is the username to connect to the CloudSQL instance. When using a service account
	// ending in `iam.gserviceaccount.com`, the username must be truncated after the `.iam` to
	// match the IAM user.
	user string

	// password is the password to connect to the CloudSQL instance. This value is only used when
	// the user type is `BUILT_IN` and it is not supported except for internal use cases.
	password string

	// userType is the type of user to connect to the CloudSQL instance. The user type must be one
	// of the values in the `cloudsql.User_SqlUserType_name` map.
	userType string

	// identifier is the CloudSQL instance's connection identifier in teh format
	// `project:region:instance`. This value is only used internally and should not be set
	// manually.
	identifier string

	// creds is the Google Cloud credentials to use for the connection. If not provided, the
	// default credentials from the runtime environment will be used.
	creds *google.Credentials
}

// connectionIdentifier returns the connection identifier for the CloudSQL instance. The connection
// identifier is in the format `project:region:instance`.
func (t *connectionConfig) connectionIdentifier(ctx context.Context) (string, error) {
	if t.identifier != "" {
		return t.identifier, nil
	}

	var err error
	if t.creds == nil {
		t.creds, err = cloud.DefaultCredentials(ctx)
		if err != nil {
			return "", err
		}
	}

	if t.project == "" {
		t.project, err = cloud.ProjectIdFromCredentials(t.creds)
		if err != nil {
			return "", errors.Join(ErrProjectNotProvidedNotFound, err)
		}
	}

	var instances []*sqladmin.DatabaseInstance
	var identifier string

	if t.region != "" {

		instances, err = listCloudSQLInstances(ctx, t.creds, t.project)
		if err != nil {
			return "", err
		}
		found := false
		for _, instance := range instances {
			if instance.Name == t.instance && instance.Region == t.region {
				if found {
					return "", fmt.Errorf(
						"database instance was found multiple times: %v", t.instance,
					)
				}
				found = true
				identifier = fmt.Sprintf("%s:%s:%s", t.project, instance.Region, instance.Name)
				continue
			}
		}
	}

	if t.region == "" {

		instances, err = listCloudSQLInstances(ctx, t.creds, t.project)
		if err != nil {
			return "", errors.Join(ErrRegionNotProvidedNotFound, err)
		}
		found := false
		for _, instance := range instances {
			if instance.Name == t.instance {
				if found {
					return "", fmt.Errorf(
						"database instance was found multiple times: %v: %w", t.instance, ErrRegionNotProvidedNotFound,
					)
				}
				found = true
				t.region = instance.Region
				identifier = fmt.Sprintf("%s:%s:%s", t.project, instance.Region, instance.Name)
				continue
			}
		}
	}

	if identifier == "" {
		return "", fmt.Errorf("database instance not found: %s", t.instance)
	}

	t.identifier = identifier
	return identifier, nil
}

// listCloudSQLInstances lists CloudSQL instances for a given project ID using the provided credentials.
func listCloudSQLInstances(
	ctx context.Context, creds *google.Credentials, projectID string,
) ([]*sqladmin.DatabaseInstance, error) {
	// Initialize the CloudSQL Admin service
	sqlService, err := sqladmin.NewService(
		ctx, option.WithCredentials(creds), option.WithUserAgent(blackstart.UserAgent),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create SQL Admin service: %w", err)
	}

	// List instances for the given project ID
	instancesList, err := sqlService.Instances.List(projectID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("failed to list instances: %w", err)
	}

	return instancesList.Items, nil
}

// postgresAdcIamUser returns the IAM user for the current ADC or workload identity in the format
// expected by CloudSQL Postgres.
func postgresAdcIamUser(ctx context.Context) (string, error) {
	u, err := cloud.AdcIamUser(ctx)
	if err != nil {
		return "", err
	}
	u = strings.TrimSuffix(u, ".gserviceaccount.com")
	return u, nil
}

// postgresIamUser returns the IAM user for the given credentials in the format expected by CloudSQL
// Postgres.
func postgresIamUser(ctx context.Context, creds *google.Credentials) (string, error) {
	iamUser, err := cloud.IamUser(ctx, creds)
	if err != nil {
		return "", err
	}
	u := strings.TrimSuffix(iamUser, ".gserviceaccount.com")
	return u, nil
}

// iamUserType returns the user type for the given credentials.
func iamUserType(creds *google.Credentials) string {
	type tokenUserSchema struct {
		UserType string `json:"type"`
	}
	var tokenUser tokenUserSchema
	err := json.Unmarshal(creds.JSON, &tokenUser)
	if err == nil && tokenUser.UserType != "authorized_user" {
		return userCloudIamServiceAccount
	}
	return userCloudIamUser
}

// cloudsqlPostgresIamDsn returns the DSN for connecting to a CloudSQL Postgres instance using IAM
// authentication. This DSN is used with the cloud.google.com/go/cloudsqlconn package of registered
// drivers.
func cloudsqlPostgresIamDsn(instanceIdentifier, dbname, username string) string {
	return fmt.Sprintf(
		"host=%s user=%s dbname=%s sslmode=disable",
		instanceIdentifier, username, dbname,
	)
}

// cloudsqlPostgresBuiltInDsn returns the DSN for connecting to a CloudSQL Postgres instance using
// built-in authentication. This DSN is used with the cloud.google.com/go/cloudsqlconn package of
// registered drivers. The password is included in the DSN and this is not supported for external
// use cases.
func cloudsqlPostgresBuiltInDsn(instanceIdentifier, dbname, username, password string) string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s sslmode=disable",
		instanceIdentifier, username, password, dbname,
	)
}
