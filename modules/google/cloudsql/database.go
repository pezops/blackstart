package cloudsql

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"strings"

	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/sqladmin/v1"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/modules/google/cloud"
	"github.com/pezops/blackstart/util"
)

func init() {
	blackstart.RegisterModule("google_cloudsql_database", NewCloudSqlDatabase)
}

var _ blackstart.Module = &database{}
var requiredCloudSQLDatabaseParameters = []string{inputInstance, inputDatabase}

// NewCloudSqlDatabase creates a new instance of the Cloud SQL database module.
func NewCloudSqlDatabase() blackstart.Module {
	return &database{}
}

// database manages a database on a Cloud SQL instance.
type database struct {
	target     *connectionConfig
	charset    string
	collation  string
	sqlService *sqladmin.Service
	// runtime provides injectable Cloud SQL Admin API dependencies.
	runtime *cloudSQLRuntime
}

// Info returns metadata describing the Cloud SQL database module.
func (d *database) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   "google_cloudsql_database",
		Name: "Google Cloud SQL database",
		Description: util.CleanString(
			`
Ensures that a database exists on a Google Cloud SQL for PostgreSQL or MySQL instance using the
Cloud SQL Admin API.

**Notes**

- This module does not create or delete the Cloud SQL instance.
- This module does not manage database ownership or privileges.
- Cloud SQL for SQL Server is not supported by this module.
- '''charset''' and '''collation''' are only supported for MySQL. When unset, Cloud SQL API defaults are used.
`,
		),
		Requirements: []string{
			"The Cloud SQL instance must exist.",
			"The [Cloud SQL Admin API](https://docs.cloud.google.com/sql/docs/mysql/admin-api) must be enabled on the project.",
			"The Blackstart service account must have permission to manage databases on the instance. The suggested pre-defined role is [`roles/cloudsql.admin`](https://docs.cloud.google.com/iam/docs/roles-permissions/cloudsql#cloudsql.admin).",
		},
		Inputs: map[string]blackstart.InputValue{
			inputInstance: {
				Description: "Cloud SQL instance ID.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputDatabase: {
				Description: "Database name to manage.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputProject: {
				Description: "Google Cloud project ID. If not provided, the current project will be used.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
			inputRegion: {
				Description: "Google Cloud region for the Cloud SQL instance. Accepted for consistency with other Cloud SQL modules.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
			inputCharset: {
				Description: "Optional MySQL charset value. When omitted, the Cloud SQL API default is used.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
			inputCollation: {
				Description: "Optional MySQL collation value. When omitted, the Cloud SQL API default is used.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputDatabase: {
				Description: "The database name that was created or managed.",
				Type:        reflect.TypeFor[string](),
			},
		},
		Examples: map[string]string{
			"Create a database": `id: create-app-db
module: google_cloudsql_database
inputs:
  instance: my-cloudsql-instance
  database: app`,
			"Create a MySQL database with charset and collation": `id: create-app-db
module: google_cloudsql_database
inputs:
  instance: my-cloudsql-instance
  database: app
  charset: utf8mb4
  collation: utf8mb4_0900_ai_ci`,
		},
	}
}

// Validate checks whether an operation contains valid Cloud SQL database inputs.
func (d *database) Validate(op blackstart.Operation) error {
	for _, p := range requiredCloudSQLDatabaseParameters {
		if _, ok := op.Inputs[p]; !ok {
			return fmt.Errorf("missing required parameter: %s", p)
		}
	}

	if err := validateStaticStringInput(op, inputInstance); err != nil {
		return err
	}
	if err := validateStaticStringInput(op, inputDatabase); err != nil {
		return err
	}
	if err := validateOptionalStaticStringInput(op, inputCharset); err != nil {
		return err
	}
	if err := validateOptionalStaticStringInput(op, inputCollation); err != nil {
		return err
	}

	return nil
}

// Check reports whether the target Cloud SQL database is in the requested state.
func (d *database) Check(ctx blackstart.ModuleContext) (bool, error) {
	if err := d.setup(ctx); err != nil {
		return false, err
	}

	exists, err := d.databaseExists(ctx)
	if err != nil {
		return false, err
	}

	var res bool
	if ctx.DoesNotExist() {
		res = !exists
	} else {
		res = exists
	}

	if res && !ctx.DoesNotExist() {
		if err = ctx.Output(outputDatabase, d.target.database); err != nil {
			return false, err
		}
	}

	return res, nil
}

// Set reconciles the target Cloud SQL database to the requested state.
func (d *database) Set(ctx blackstart.ModuleContext) error {
	if err := d.setup(ctx); err != nil {
		return err
	}

	if ctx.DoesNotExist() {
		return d.deleteDatabase(ctx)
	}

	if err := d.createDatabase(ctx); err != nil {
		return err
	}

	return ctx.Output(outputDatabase, d.target.database)
}

// setup initializes the target configuration and SQL Admin service for the database module.
func (d *database) setup(ctx blackstart.ModuleContext) error {
	var err error
	d.target = &connectionConfig{}

	d.target.instance, err = blackstart.ContextInputAs[string](ctx, inputInstance, true)
	if err != nil {
		return err
	}
	if d.target.instance == "" {
		return fmt.Errorf("instance cannot be empty")
	}

	d.target.database, err = blackstart.ContextInputAs[string](ctx, inputDatabase, true)
	if err != nil {
		return err
	}
	if d.target.database == "" {
		return fmt.Errorf("database cannot be empty")
	}

	d.target.project, err = blackstart.ContextInputAs[string](ctx, inputProject, false)
	if err != nil {
		return err
	}
	if d.target.project == "" {
		var creds *google.Credentials
		d.target.project, creds, err = cloud.CurrentProject(ctx)
		if err != nil {
			return err
		}
		d.target.creds = creds
	}

	d.target.region, err = blackstart.ContextInputAs[string](ctx, inputRegion, false)
	if err != nil {
		return err
	}

	d.charset, err = blackstart.ContextInputAs[string](ctx, inputCharset, false)
	if err != nil {
		return err
	}
	d.collation, err = blackstart.ContextInputAs[string](ctx, inputCollation, false)
	if err != nil {
		return err
	}

	d.runtime = cloudSQLRuntimeOrDefault(d.runtime)
	d.sqlService, err = d.runtime.newSQLAdminService(ctx)
	if err != nil {
		return fmt.Errorf("failed to create SQL Admin service: %w", err)
	}

	instance, err := d.sqlService.Instances.Get(d.target.project, d.target.instance).Context(ctx).Do()
	if err != nil {
		return fmt.Errorf("failed to get instance %s in project %s: %w", d.target.instance, d.target.project, err)
	}
	d.target.engine = instanceEngine(instance.DatabaseVersion)
	d.target.databaseVersion = instance.DatabaseVersion
	d.target.region = instance.Region
	d.target.identifier = fmt.Sprintf("%s:%s:%s", d.target.project, instance.Region, d.target.instance)

	switch d.target.engine {
	case "POSTGRES":
		if strings.TrimSpace(d.charset) != "" || strings.TrimSpace(d.collation) != "" {
			return fmt.Errorf("charset and collation are only supported for MySQL Cloud SQL databases")
		}
	case "MYSQL":
	default:
		return fmt.Errorf("the Cloud SQL engine %q is not supported by google_cloudsql_database", d.target.engine)
	}

	return nil
}

// databaseExists reports whether the target database exists.
func (d *database) databaseExists(ctx context.Context) (bool, error) {
	_, err := d.sqlService.Databases.Get(
		d.target.project,
		d.target.instance,
		d.target.database,
	).Context(ctx).Do()
	if err == nil {
		return true, nil
	}
	var apiErr *googleapi.Error
	if errors.As(err, &apiErr) && apiErr.Code == http.StatusNotFound {
		return false, nil
	}
	return false, fmt.Errorf("failed to get database %s in instance %s: %w", d.target.database, d.target.instance, err)
}

// createDatabase creates the target Cloud SQL database.
func (d *database) createDatabase(ctx context.Context) error {
	db := &sqladmin.Database{
		Name:     d.target.database,
		Project:  d.target.project,
		Instance: d.target.instance,
	}
	if strings.TrimSpace(d.charset) != "" {
		db.Charset = d.charset
	}
	if strings.TrimSpace(d.collation) != "" {
		db.Collation = d.collation
	}

	result, err := d.sqlService.Databases.Insert(d.target.project, d.target.instance, db).Context(ctx).Do()
	if err != nil {
		return err
	}
	if result == nil {
		return fmt.Errorf("database insert result was empty")
	}
	if result.HTTPStatusCode < 200 || result.HTTPStatusCode >= 300 {
		return fmt.Errorf("status code error while inserting database: %d", result.HTTPStatusCode)
	}
	return nil
}

// deleteDatabase deletes the target Cloud SQL database.
func (d *database) deleteDatabase(ctx context.Context) error {
	result, err := d.sqlService.Databases.Delete(
		d.target.project,
		d.target.instance,
		d.target.database,
	).Context(ctx).Do()
	if err != nil {
		return err
	}
	if result == nil {
		return fmt.Errorf("database delete result was empty")
	}
	if result.HTTPStatusCode < 200 || result.HTTPStatusCode >= 300 {
		return fmt.Errorf("status code error while deleting database: %d", result.HTTPStatusCode)
	}
	return nil
}

// validateStaticStringInput validates a required static string input when it is statically known.
func validateStaticStringInput(op blackstart.Operation, key string) error {
	input := op.Inputs[key]
	if !input.IsStatic() {
		return nil
	}
	value, err := blackstart.InputAs[string](input, true)
	if err != nil {
		return fmt.Errorf("invalid %s: %w", key, err)
	}
	if value == "" {
		return fmt.Errorf("%s cannot be empty", key)
	}
	return nil
}

// validateOptionalStaticStringInput validates an optional static string input when it is configured.
func validateOptionalStaticStringInput(op blackstart.Operation, key string) error {
	input, ok := op.Inputs[key]
	if !ok || !input.IsStatic() {
		return nil
	}
	if _, err := blackstart.InputAs[string](input, false); err != nil {
		return fmt.Errorf("invalid %s: %w", key, err)
	}
	return nil
}
