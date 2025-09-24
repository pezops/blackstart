package postgres

import (
	"bytes"
	"database/sql"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"text/template"

	"github.com/pezops/blackstart"
)

func init() {
	blackstart.RegisterModule("postgres_grant", NewPostgresGrant)
}

// scope is the Resource-level scope where a grant is applied. This might be an instance, Schema, table,
// or database.
type scope string

// scopes contains the valid scopes for a grant operation that can be referenced in code.
var scopes = struct {
	instance scope
	schema   scope
	table    scope
	database scope
}{
	instance: "INSTANCE",
	schema:   "SCHEMA",
	table:    "TABLE",
	database: "DATABASE",
}

// scopesList contains the valid scopes for a grant operation that can be referenced in code.
var scopesList = []scope{scopes.instance, scopes.schema, scopes.table, scopes.database}

var _ blackstart.Module = &grantModule{}
var requiredGrantParameters = []string{inputRole, inputPermission, inputConnection}
var grantSchemaPermissions = []string{"CREATE", "USAGE", "ALL"}
var grantDatabasePermissions = []string{"CREATE", "CONNECT", "TEMPORARY", "TEMP", "ALL"}
var requiredRoleParameters = []string{inputName}

func NewPostgresGrant() blackstart.Module {
	return &grantModule{}
}

// grant contains the information for a grant operation. Currently, this only supports grants that
// are applied to the database instance (role membership), database, Schema, or table.
type grant struct {
	// Role or username that will have the grant assigned
	Role string
	// Permission or role to be assigned to the Role. Depending on the resource scope, the valid
	// permissions may vary. For role membership, where one role is being granted to another,
	// this is only valid for the instance grant scope.
	Permission string
	// Schema is the name of a Postgres schema where the Permission is to be applied. Defaults to
	// the "public" schema.
	Schema string
	// Resource is the name of the resource for the Permission to be applied. This might be a database
	// name, table name, or schema name.
	Resource string
	// Scope is the type of Resource where the Permission is to be applied. This might be a database,
	// table, or schema.
	Scope string
}

// newGrant creates a new grant object from the module context inputs. It validates the inputs and
// returns an error if any required inputs are missing or invalid.
func newGrant(mctx blackstart.ModuleContext) (*grant, error) {
	var err error

	target := &grant{}
	role, err := mctx.Input(inputRole)
	if err != nil {
		return nil, err
	}
	if role.String() == "" {
		return nil, fmt.Errorf("role cannot be empty")
	}
	target.Role = role.String()

	perm, err := mctx.Input(inputPermission)
	if err != nil {
		return nil, err
	}
	if perm.String() == "" {
		return nil, fmt.Errorf("permission cannot be empty")
	}
	target.Permission = perm.String()

	schema, _ := mctx.Input(inputSchema)
	if schema != nil {
		target.Schema = schema.String()
	}
	if target.Schema != "" {
		err = validatePostgresIdentifier(target.Schema)
		if err != nil {
			return nil, err
		}
	}

	resource, _ := mctx.Input(inputResource)
	if resource != nil {
		target.Resource = resource.String()
	}
	if target.Resource != "" {
		err = validatePostgresIdentifier(target.Resource)
		if err != nil {
			return nil, err
		}
	}

	scopeInput, _ := mctx.Input(inputScope)
	if scopeInput != nil {
		target.Scope = scopeInput.String()
	}

	if target.Scope == "" {
		target.Scope = "instance"
	}

	var s scope
	s, err = stringToScope(target.Scope)
	if err != nil {
		return nil, err
	}
	target.Scope = string(s)

	return target, nil
}

type grantModule struct {
	db     *sql.DB
	target *grant
}

func (g *grantModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:          "postgres_grant",
		Name:        "PostgreSQL grant",
		Description: "Ensures that a Postgres Role has the specified Permission on a Resource.",
		Inputs: map[string]blackstart.InputValue{
			inputConnection: {
				Description: "database connection to the managed Postgres instance.",
				Type:        reflect.TypeOf(&sql.DB{}),
				Required:    true,
			},
			inputRole: {
				Description: "Role or username that will have the grant assigned.",
				Type:        reflect.TypeOf(""),
				Required:    true,
			},
			inputPermission: {
				Description: "Permission or Role to be assigned to the Role. Depending on the Resource Scope, the valid permissions may vary.",
				Type:        reflect.TypeOf(""),
				Required:    true,
			},
			inputSchema: {
				Description: "Id of a Postgres Schema where the Permission is to be applied.",
				Type:        reflect.TypeOf(""),
				Required:    false,
			},
			inputResource: {
				Description: "Id of the Resource for the Permission to be applied. This might be a database Name, table Name, or Schema Name.",
				Type:        reflect.TypeOf(""),
				Required:    false,
			},
			inputScope: {
				Description: "Scope of the Resource where the Permission is to be applied. This might be a database, table, or Schema.",
				Type:        reflect.TypeOf(""),
				Required:    false,
			},
		},
		Outputs: map[string]blackstart.OutputValue{},
		Examples: map[string]string{
			"grant Role membership": `id: grant-Role-membership
module: postgres_grant
inputs:
  connection:
    from_dependency:
      id: manage-instance
      output: connection
  Role: my-user
  Permission: my-other-Role`,
			"grant Schema usage": `id: grant-Schema-usage
module: postgres_grant
inputs:
  connection:
    from_dependency:
      id: manage-instance
      output: connection
  Role: my-user
  Permission: USAGE
  Scope: SCHEMA
  Resource: my-Schema`,
		},
	}
}

func (g *grantModule) Validate(op blackstart.Operation) error {
	for _, p := range requiredGrantParameters {
		if o, ok := op.Inputs[p]; !ok {
			return fmt.Errorf("missing required parameter: %s", p)
		} else {
			if !o.IsStatic() {
				continue
			}
			v, err := o.Auto()
			if err != nil {
				return err
			}
			switch x := v.(type) {
			case string:
				if x == "" {
					return fmt.Errorf("parameter %s cannot be empty", p)
				}
			default:
				if x == nil {
					return fmt.Errorf("parameter %s cannot be nil", p)
				}

			}
		}
	}

	return nil
}

func (g *grantModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	var err error
	err = g.setup(ctx)
	if err != nil {
		return false, err
	}

	g.target, err = newGrant(ctx)
	if err != nil {
		return false, err
	}

	// Check for required runtime params
	ok, err := checkGrantRuntimeParams(ctx)
	if !ok {
		return false, err
	}

	query, queryParams, err := getGrantExistsQuery(g.target)
	if err != nil {
		return false, fmt.Errorf("error getting grant query: %w", err)
	}

	// Execute the query
	var exists bool
	err = g.db.QueryRowContext(ctx, query, queryParams...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("error checking grant: %w", err)
	}

	// Capture the result
	if ctx.DoesNotExist() {
		return !exists, nil
	}
	return exists, nil
}

func (g *grantModule) Set(ctx blackstart.ModuleContext) error {
	var err error
	err = g.setup(ctx)
	if err != nil {
		return err
	}

	// Check for required runtime params
	ok, err := checkGrantRuntimeParams(ctx)
	if !ok {
		return err
	}

	g.target, err = newGrant(ctx)
	if err != nil {
		return err
	}

	if ctx.DoesNotExist() {
		// If the does not exist flag is set, the Check() has determined the grant exists and
		// should be deleted.
		query, queryParams := getGrantRevokeQuery(g.target)

		// Execute the query
		_, err = g.db.ExecContext(ctx, query, queryParams...)
		if err != nil {
			return fmt.Errorf("error revoking grant: %w", err)
		}
		return nil
	}

	query, queryParams, err := getGrantSetQuery(g.target)
	if err != nil {
		return fmt.Errorf("error getting grant query: %w", err)
	}

	// Execute the query
	_, err = g.db.ExecContext(ctx, query, queryParams...)
	if err != nil {
		return fmt.Errorf("error setting grant: %w", err)
	}

	return nil
}

// setup initializes the grantModule by extracting the database connection from the module context.
func (g *grantModule) setup(ctx blackstart.ModuleContext) error {
	var connVal any
	connInput, err := ctx.Input(inputConnection)
	if err != nil {
		return err
	}

	connVal, err = connInput.Auto()
	if err != nil {
		return err
	}
	conn, ok := connVal.(*sql.DB)
	if !ok {
		return fmt.Errorf("invalid connection input type: %T", connVal)
	}
	g.db = conn

	return nil
}

// getGrantExistsQuery constructs the SQL query to check if a grant exists based on the target grant
// object's Scope. It returns the query string, query parameters, and any error encountered.
//
// goland:noinspection SqlNoDataSourceInspection
func getGrantExistsQuery(target *grant) (string, []interface{}, error) {
	// Construct the SQL query

	grantScope, err := stringToScope(target.Scope)
	if err != nil {
		return "", nil, err
	}

	switch grantScope {
	case scopes.instance:
		queryParams := []interface{}{target.Permission, target.Role}
		return getGrantInstanceQuery, queryParams, nil
	case scopes.database:
		queryParams := []interface{}{target.Permission, target.Role, target.Resource}
		return getGrantDatabaseQuery, queryParams, nil
	case scopes.schema:
		queryParams := []interface{}{target.Permission, target.Role, target.Schema}
		return getGrantSchemaQuery, queryParams, nil
	case scopes.table:
		queryParams := []interface{}{target.Permission, target.Role, target.Resource, target.Schema}
		return getGrantTableQuery, queryParams, nil
	default:
		return "", nil, fmt.Errorf("no query for Scope: %s", target.Scope)
	}
}

// stringToScope converts a string to a scope type. If the string is not a valid scope, an error is
// returned.
func stringToScope(s string) (scope, error) {
	s = strings.ToUpper(s)

	for _, sc := range scopesList {
		if s == string(sc) {
			return sc, nil
		}
	}
	return "", fmt.Errorf("invalid Scope: %s", s)
}

// getGrantSetQuery constructs the SQL query to set a grant based on the target grant object's
// Scope. It returns the query string, query parameters, and any error encountered.
//
// goland:noinspection SqlNoDataSourceInspection
func getGrantSetQuery(target *grant) (string, []interface{}, error) {
	grantScope, err := stringToScope(target.Scope)
	if err != nil {
		return "", nil, err
	}

	perm := strings.ToUpper(target.Permission)
	var tmpl *template.Template

	switch grantScope {
	case scopes.instance:
		tmpl, err = template.New("setGrantInstance").Parse(setGrantInstanceTemplate)
	case scopes.database:
		if !slices.Contains(grantDatabasePermissions, perm) {
			return "", nil, fmt.Errorf("invalid database Permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantDatabase").Parse(setGrantDatabaseTemplate)
	case scopes.schema:
		if !slices.Contains(grantSchemaPermissions, perm) {
			return "", nil, fmt.Errorf("invalid Schema Permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantSchema").Parse(setGrantSchemaTemplate)
	case scopes.table:
		tmpl, err = template.New("setGrantTable").Parse(setGrantTableTemplate)
	default:
		return "", nil, fmt.Errorf("no query for Scope: %s", target.Scope)
	}

	if err != nil {
		return "", nil, err
	}

	var queryBuffer bytes.Buffer
	err = tmpl.Execute(&queryBuffer, target)
	if err != nil {
		return "", nil, err
	}

	return queryBuffer.String(), nil, nil
}

// getGrantRevokeQuery constructs the SQL query to revoke a grant based on the target grant object's
// Scope. It returns the query string and query parameters.
//
// goland:noinspection SqlNoDataSourceInspection
func getGrantRevokeQuery(target *grant) (string, []interface{}) {
	// Construct the SQL query

	switch target.Scope {
	case "instance":
		query := `
    REVOKE $1 FROM $2;
    `
		queryParams := []interface{}{target.Permission, target.Role}
		return query, queryParams
	case "database":
		query := `
    REVOKE $1 FROM $2 IN DATABASE $3;
    `
		queryParams := []interface{}{target.Permission, target.Role, target.Resource}
		return query, queryParams
	case "schema":
		query := `
    REVOKE $1 FROM $2 IN SCHEMA $3;
    `
		queryParams := []interface{}{target.Permission, target.Role, target.Resource}
		return query, queryParams
	case "table":
		query := `
    REVOKE $1 FROM $2 ON TABLE $3;
    `
		queryParams := []interface{}{target.Permission, target.Role, target.Resource}
		return query, queryParams
	}
	return "", nil
}

// checkGrantRuntimeParams checks that all required runtime parameters are present and valid. It also
// verifies that the database connection is of the correct type.
func checkGrantRuntimeParams(ctx blackstart.ModuleContext) (bool, error) {
	var err error
	var o blackstart.Input
	// Check runtime parameters
	for _, p := range requiredGrantParameters {
		o, err = ctx.Input(p)
		if err != nil {
			return false, fmt.Errorf("missing required parameter: %s", p)
		} else {
			if !o.IsStatic() {
				continue
			}
			var v any
			v, err = o.Auto()
			if err != nil {
				return false, err
			}
			switch x := v.(type) {
			case string:
				if x == "" {
					return false, fmt.Errorf("parameter %s cannot be empty", p)
				}
			default:
				if x == nil {
					return false, fmt.Errorf("parameter %s cannot be nil", p)
				}

			}
		}
	}

	// Check the connection
	o, err = ctx.Input(inputConnection)
	if err != nil {
		return false, err
	}
	connObj, err := o.Auto()
	if err != nil {
		return false, err
	}
	_, ok := connObj.(*sql.DB)
	if !ok {
		return false, fmt.Errorf("invalid connection input type: %T", connObj)
	}

	return true, nil
}
