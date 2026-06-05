package cloudsql

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"cloud.google.com/go/cloudsqlconn"
	cloudsqlmysql "cloud.google.com/go/cloudsqlconn/mysql/mysql"
	"cloud.google.com/go/cloudsqlconn/postgres/pgxv5"
	gomysql "github.com/go-sql-driver/mysql"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/option"
	"google.golang.org/api/sqladmin/v1"
	cloudsqlv1 "google.golang.org/genproto/googleapis/cloud/sql/v1"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/modules/google/cloud"
)

// cloudSQLRuntime provides injectable Cloud SQL Admin API and database connection dependencies.
type cloudSQLRuntime struct {
	newSQLAdminService func(context.Context) (*sqladmin.Service, error)
	openDB             func(string, string) (*sql.DB, error)
}

// defaultCloudSQLRuntime creates the production Cloud SQL runtime.
func defaultCloudSQLRuntime() *cloudSQLRuntime {
	return &cloudSQLRuntime{
		newSQLAdminService: func(ctx context.Context) (*sqladmin.Service, error) {
			return sqladmin.NewService(ctx, option.WithUserAgent(blackstart.UserAgent))
		},
		openDB: sql.Open,
	}
}

// cloudSQLRuntimeOrDefault returns runtime when configured, or the production runtime otherwise.
func cloudSQLRuntimeOrDefault(runtime *cloudSQLRuntime) *cloudSQLRuntime {
	if runtime == nil {
		return defaultCloudSQLRuntime()
	}
	return runtime
}

const (
	modulePackage = "google.cloudsql"

	// sqlDriverPostgresIam is the driver name for connecting to Cloud SQL for PostgreSQL instances
	// using IAM credentials.
	sqlDriverPostgresIam = "cloudsql-postgres-iam"

	// sqlDriverPostgresIamPrivateIp is the driver name for connecting to Cloud SQL for PostgreSQL instances
	// using IAM credentials connecting via a VPC and private IP address.
	sqlDriverPostgresIamPrivateIp = "cloudsql-postgres-iam-private"

	// sqlDriverPostgres is the driver name for connecting to Cloud SQL for PostgreSQL instances using
	// built-in / static credentials.
	sqlDriverPostgres = "cloudsql-postgres"

	// sqlDriverPostgresPrivateIp is the driver name for connecting to Cloud SQL for PostgreSQL instances using
	// built-in / static credentials connecting via a VPC and private IP address.
	sqlDriverPostgresPrivateIp = "cloudsql-postgres-private"

	// sqlDriverMySQLIam is the driver name for connecting to Cloud SQL for MySQL instances
	// using IAM credentials.
	sqlDriverMySQLIam = "cloudsql-mysql-iam"

	// sqlDriverMySQLIamPrivateIp is the driver name for connecting to Cloud SQL for MySQL
	// instances using IAM credentials via a VPC and private IP address.
	sqlDriverMySQLIamPrivateIp = "cloudsql-mysql-iam-private"

	// sqlDriverMySQL is the driver name for connecting to Cloud SQL for MySQL instances using
	// built-in / static credentials.
	sqlDriverMySQL = "cloudsql-mysql"

	// sqlDriverMySQLPrivateIp is the driver name for connecting to Cloud SQL for MySQL instances
	// using built-in / static credentials via a VPC and private IP address.
	sqlDriverMySQLPrivateIp = "cloudsql-mysql-private"

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
	inputCharset        = "charset"
	inputCollation      = "collation"

	outputUser       = "user"
	outputDatabase   = "database"
	outputConnection = "connection"
)

func init() {
	blackstart.RegisterPathName("cloudsql", "Cloud SQL")

	// Register global drivers that can be used by the database/sql package with the new package
	// to connect to Cloud SQL, cloud.google.com/go/cloudsqlconn. Its PostgreSQL and MySQL
	// subpackages connect without the need for the Cloud SQL proxy.
	//
	// To leverage a credentials file, a driver per credential is needed that specifies the file
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
	_, _ = cloudsqlmysql.RegisterDriver(sqlDriverMySQLIam, cloudsqlconn.WithIAMAuthN())
	_, _ = cloudsqlmysql.RegisterDriver(
		sqlDriverMySQLIamPrivateIp,
		cloudsqlconn.WithIAMAuthN(),
		cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()),
	)
	_, _ = cloudsqlmysql.RegisterDriver(sqlDriverMySQL)
	_, _ = cloudsqlmysql.RegisterDriver(
		sqlDriverMySQLPrivateIp,
		cloudsqlconn.WithDefaultDialOptions(cloudsqlconn.WithPrivateIP()),
	)

	// Quick check to make sure these shorter constants here did not change from the upstream values.
	if userBuiltIn != cloudsqlv1.User_SqlUserType_name[int32(cloudsqlv1.User_BUILT_IN)] ||
		userCloudIamUser != cloudsqlv1.User_SqlUserType_name[int32(cloudsqlv1.User_CLOUD_IAM_USER)] ||
		userCloudIamServiceAccount != cloudsqlv1.User_SqlUserType_name[int32(cloudsqlv1.User_CLOUD_IAM_SERVICE_ACCOUNT)] {
		panic("cloudsql user type names do not match")
	}
}

// ErrRegionNotProvidedNotFound indicates a region was neither provided nor discovered.
var ErrRegionNotProvidedNotFound = errors.New("region not provided and not found")

// ErrProjectNotProvidedNotFound indicates a project was neither provided nor discovered.
var ErrProjectNotProvidedNotFound = errors.New("project not provided and not found")

// cloudSQLServiceAccountIdentityPattern matches full and Cloud SQL-normalized service-account identities.
var cloudSQLServiceAccountIdentityPattern = regexp.MustCompile(
	`^[^@\s]+@[^@\s]+\.iam(?:\.gserviceaccount\.com)?$`,
)

// connectionConfig is the configuration for a connection to a Cloud SQL instance. It provides some
// convenience methods for working with the Cloud SQL instance.
type connectionConfig struct {
	// instance is the name of the Cloud SQL instance.
	instance string

	// database is the name of the database to connect to on the Cloud SQL instance.
	database string

	// project is the Google Cloud project ID. This value will attempt to be determined from the
	// credentials if not provided.
	project string

	// region is the region where the Cloud SQL instance is located. If not provided, the region
	// will attempt to be determined using the sqladmin API.
	region string

	// engine and databaseVersion are inferred from Cloud SQL instance metadata.
	engine          string
	databaseVersion string

	// user is the username to connect to the Cloud SQL instance. When using a service account
	// ending in `iam.gserviceaccount.com`, the username must be truncated after the `.iam` to
	// match the IAM user.
	user string

	// password is the password to connect to the Cloud SQL instance. This value is only used when
	// the user type is `BUILT_IN` and it is not supported except for internal use cases.
	password string

	// userType is the type of user to connect to the Cloud SQL instance. The user type must be one
	// of the values in the `cloudsql.User_SqlUserType_name` map.
	userType string

	// identifier is the Cloud SQL instance's connection identifier in teh format
	// `project:region:instance`. This value is only used internally and should not be set
	// manually.
	identifier string

	// creds is the Google Cloud credentials to use for the connection. If not provided, the
	// default credentials from the runtime environment will be used.
	creds *google.Credentials
}

// connectionIdentifier returns the connection identifier for the Cloud SQL instance. The connection
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

// listCloudSQLInstances lists Cloud SQL instances for a given project ID using the provided credentials.
func listCloudSQLInstances(
	ctx context.Context, creds *google.Credentials, projectID string,
) ([]*sqladmin.DatabaseInstance, error) {
	// Initialize the Cloud SQL Admin service
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
// expected by Cloud SQL for PostgreSQL.
func postgresAdcIamUser(ctx context.Context) (string, error) {
	u, err := cloud.AdcIamUser(ctx)
	if err != nil {
		return "", err
	}
	u = strings.TrimSuffix(u, ".gserviceaccount.com")
	return u, nil
}

// postgresIamUser returns the IAM user for the given credentials in the format expected by Cloud SQL
// for PostgreSQL.
func postgresIamUser(ctx context.Context, creds *google.Credentials) (string, error) {
	iamUser, err := cloud.IamUser(ctx, creds)
	if err != nil {
		return "", err
	}
	u := strings.TrimSuffix(iamUser, ".gserviceaccount.com")
	return u, nil
}

// mysqlIamUser returns the local database username Cloud SQL for MySQL derives from an IAM
// identity. Cloud SQL truncates the identity at the first @ character.
func mysqlIamUser(iamIdentity string) (string, error) {
	identity := strings.TrimSpace(iamIdentity)
	if identity == "" {
		return "", fmt.Errorf("IAM identity cannot be empty")
	}
	if identity != strings.ToLower(identity) {
		return "", fmt.Errorf("MySQL IAM identity must be lowercase: %s", identity)
	}
	username, _, found := strings.Cut(identity, "@")
	if !found || username == "" {
		return "", fmt.Errorf("MySQL IAM identity must be an email address: %s", identity)
	}
	return username, nil
}

// instanceEngine returns the normalized database engine for a Cloud SQL database version.
func instanceEngine(databaseVersion string) string {
	version := strings.ToUpper(strings.TrimSpace(databaseVersion))
	switch {
	case strings.HasPrefix(version, "POSTGRES"):
		return "POSTGRES"
	case strings.HasPrefix(version, "MYSQL"):
		return "MYSQL"
	case strings.HasPrefix(version, "SQLSERVER"):
		return "SQLSERVER"
	default:
		return "UNKNOWN"
	}
}

// mysqlManagedInstanceSupported reports whether a MySQL version supports managed-instance role administration.
func mysqlManagedInstanceSupported(databaseVersion string) bool {
	major, _, err := mysqlVersionNumbers(databaseVersion)
	return err == nil && major >= 8
}

// mysqlIamUserSupported reports whether a MySQL version supports IAM database users.
func mysqlIamUserSupported(databaseVersion string) bool {
	major, minor, err := mysqlVersionNumbers(databaseVersion)
	return err == nil && (major > 5 || major == 5 && minor >= 7)
}

// mysqlVersionNumbers parses the major and minor components from a Cloud SQL MySQL database version.
func mysqlVersionNumbers(databaseVersion string) (int, int, error) {
	parts := strings.Split(strings.ToUpper(strings.TrimSpace(databaseVersion)), "_")
	if len(parts) < 3 || parts[0] != "MYSQL" {
		return 0, 0, fmt.Errorf("invalid Cloud SQL MySQL database version: %s", databaseVersion)
	}
	major, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid Cloud SQL MySQL major version %q: %w", parts[1], err)
	}
	minor, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid Cloud SQL MySQL minor version %q: %w", parts[2], err)
	}
	return major, minor, nil
}

// instanceIamAuthenticationEnabled reports whether an engine-specific IAM authentication flag is enabled.
func instanceIamAuthenticationEnabled(instance *sqladmin.DatabaseInstance) bool {
	if instance == nil || instance.Settings == nil {
		return false
	}
	for _, flag := range instance.Settings.DatabaseFlags {
		if flag == nil {
			continue
		}
		if strings.EqualFold(flag.Name, "cloudsql.iam_authentication") ||
			strings.EqualFold(flag.Name, "cloudsql_iam_authentication") {
			if strings.EqualFold(flag.Value, "on") {
				return true
			}
		}
	}
	return false
}

// iamUserType returns the Cloud SQL IAM user type for the resolved IAM identity. It classifies
// service accounts by identity shape first, then falls back to credential JSON metadata.
func iamUserType(creds *google.Credentials, iamUser string) string {
	normalized := strings.TrimSpace(strings.ToLower(iamUser))
	if cloudSQLServiceAccountIdentityPattern.MatchString(normalized) {
		return userCloudIamServiceAccount
	}

	type tokenUserSchema struct {
		UserType string `json:"type"`
	}
	var tokenUser tokenUserSchema
	if creds == nil || len(creds.JSON) == 0 {
		return userCloudIamUser
	}
	err := json.Unmarshal(creds.JSON, &tokenUser)
	if err == nil && tokenUser.UserType != "authorized_user" {
		return userCloudIamServiceAccount
	}
	return userCloudIamUser
}

// cloudsqlPostgresIamDsn returns the DSN for connecting to a Cloud SQL for PostgreSQL instance using IAM
// authentication. This DSN is used with the cloud.google.com/go/cloudsqlconn package of registered
// drivers.
func cloudsqlPostgresIamDsn(instanceIdentifier, dbname, username string) string {
	return fmt.Sprintf(
		"host=%s user=%s dbname=%s sslmode=disable",
		instanceIdentifier, username, dbname,
	)
}

// cloudsqlPostgresBuiltInDsn returns the DSN for connecting to a Cloud SQL for PostgreSQL instance using
// built-in authentication. This DSN is used with the cloud.google.com/go/cloudsqlconn package of
// registered drivers. The password is included in the DSN and this is not supported for external
// use cases.
func cloudsqlPostgresBuiltInDsn(instanceIdentifier, dbname, username, password string) string {
	return fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s sslmode=disable",
		instanceIdentifier, username, password, dbname,
	)
}

// cloudsqlMySQLDsn returns a DSN for a registered Cloud SQL MySQL driver.
func cloudsqlMySQLDsn(driverNetwork, instanceIdentifier, dbname, username, password string) string {
	cfg := gomysql.NewConfig()
	cfg.User = username
	cfg.Passwd = password
	cfg.DBName = dbname
	cfg.Net = driverNetwork
	cfg.Addr = instanceIdentifier
	cfg.ParseTime = true
	return cfg.FormatDSN()
}
