package postgres

import (
	"bytes"
	"database/sql"
	"errors"
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
var grantTablePermissions = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN", "ALL",
}
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
	// Resource is the name of the database object for the Permission to be applied. This might be a database
	// name, table name, or schema name.
	Resource string
	// Scope is the type of Resource where the Permission is to be applied. This might be a database,
	// table, or schema.
	Scope string
}

// normalizeRequiredStringList trims required list values and rejects empty entries.
func normalizeRequiredStringList(values []string, field string) ([]string, error) {
	out := make([]string, 0, len(values))
	for i, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return nil, fmt.Errorf("input %q value[%d] cannot be empty", field, i)
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("input %q must not be empty", field)
	}
	return out, nil
}

// normalizeOptionalStringList trims optional list values and collapses empty input to a single
// empty marker so callers can handle unset optional dimensions uniformly.
func normalizeOptionalStringList(values []string) []string {
	if len(values) == 0 {
		return []string{""}
	}

	out := make([]string, 0, len(values))
	for _, v := range values {
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return []string{""}
	}
	return out
}

// normalizeScopeTargets constrains schema/resource dimensions to the valid target model for each
// scope and returns normalized schema/resource lists for expansion.
func normalizeScopeTargets(
	grantScope scope, schemas []string, resources []string,
) ([]string, []string, error) {
	switch grantScope {
	case scopes.instance:
		// INSTANCE grants model role membership only; schema/resource do not apply.
		return []string{""}, []string{""}, nil
	case scopes.database:
		// DATABASE grants target database names using the resource field.
		if len(resources) == 1 && resources[0] == "" {
			return nil, nil, fmt.Errorf("input %q must be provided when scope is DATABASE", inputResource)
		}
		return []string{""}, resources, nil
	case scopes.schema:
		// SCHEMA scope targets a schema name. Accept either resource or schema input for backward
		// compatibility, but normalize to Resource so Check/Set/Revoke use the same field.
		if len(schemas) > 1 || (len(schemas) == 1 && schemas[0] != "") {
			if len(resources) > 1 || (len(resources) == 1 && resources[0] != "") {
				if len(schemas) != len(resources) {
					return nil, nil, fmt.Errorf(
						"inputs %q and %q must match for scope SCHEMA when both are set",
						inputSchema, inputResource,
					)
				}
				for i := range schemas {
					if schemas[i] != resources[i] {
						return nil, nil, fmt.Errorf(
							"inputs %q and %q must match for scope SCHEMA when both are set",
							inputSchema, inputResource,
						)
					}
				}
				return []string{""}, resources, nil
			}
			return []string{""}, schemas, nil
		}
		if len(resources) == 1 && resources[0] == "" {
			return nil, nil, fmt.Errorf(
				"one of %q or %q must be provided when scope is SCHEMA", inputSchema, inputResource,
			)
		}
		return []string{""}, resources, nil
	case scopes.table:
		// TABLE grants target schema-qualified tables. Set the default schema to public when omitted.
		if len(schemas) == 1 && schemas[0] == "" {
			schemas = []string{"public"}
		}
		if len(resources) == 1 && resources[0] == "" {
			return nil, nil, fmt.Errorf("input %q must be provided when scope is TABLE", inputResource)
		}
		return schemas, resources, nil
	default:
		return nil, nil, fmt.Errorf("unsupported scope: %s", grantScope)
	}
}

// validateGrantRole validates a grant role identifier.
func validateGrantRole(roleName string) error {
	if roleErr := validatePostgresQuotedIdentifier(roleName); roleErr != nil {
		return fmt.Errorf("invalid role %q: %w", roleName, roleErr)
	}
	return nil
}

// validateGrantPermission validates permission tokens for the selected grant scope.
func validateGrantPermission(grantScope scope, permission string) error {
	if strings.Contains(permission, ",") {
		return fmt.Errorf(
			"invalid permission %q for scope %s: comma-separated permissions are not supported", permission, grantScope,
		)
	}

	perm := normalizeGrantPermissionToken(grantScope, permission)
	switch grantScope {
	case scopes.instance:
		// INSTANCE scope models role membership: permission is the role being granted.
		// This is the only place that raw identifiers are accepted as permissions, and these are
		// rendered as quoted identifiers in SQL, so this validates accordingly.
		if permErr := validatePostgresQuotedIdentifier(perm); permErr != nil {
			return fmt.Errorf("invalid permission %q for scope %s: %w", permission, grantScope, permErr)
		}
	case scopes.database:
		if !slices.Contains(grantDatabasePermissions, perm) {
			return fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
		}
	case scopes.schema:
		if !slices.Contains(grantSchemaPermissions, perm) {
			return fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
		}
	case scopes.table:
		if !slices.Contains(grantTablePermissions, perm) {
			return fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
		}
	default:
		return fmt.Errorf("unsupported scope: %s", grantScope)
	}
	return nil
}

// normalizeGrantPermissionToken canonicalizes permission input for validation and SQL rendering.
func normalizeGrantPermissionToken(grantScope scope, permission string) string {
	trimmed := strings.TrimSpace(permission)
	if grantScope == scopes.instance {
		return trimmed
	}
	perm := strings.ToUpper(trimmed)
	if perm == "ALL PRIVILEGES" {
		return "ALL"
	}
	return perm
}

// validateGrantPrincipalAndPermission validates role and permission inputs for the selected scope.
// This is used before SQL template rendering to prevent unsafe or unsupported values.
func validateGrantPrincipalAndPermission(grantScope scope, roleName, permission string) error {
	if err := validateGrantRole(roleName); err != nil {
		return err
	}
	return validateGrantPermission(grantScope, permission)
}

// readOptionalStringListInput reads an optional string-or-[]string input from context and returns
// whether the key was present.
func readOptionalStringListInput(
	mctx blackstart.ModuleContext, key string,
) ([]string, bool, error) {
	input, err := mctx.Input(key)
	if err != nil {
		if errors.Is(err, blackstart.ErrInputDoesNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if input == nil || !input.IsStatic() {
		return nil, true, nil
	}
	values, err := blackstart.InputAs[[]string](input, false)
	if err != nil {
		return nil, true, err
	}
	return values, true, nil
}

// expandGrantsFromContext expands single or list-valued inputs into a list of all possible
// grant targets.
func expandGrantsFromContext(mctx blackstart.ModuleContext) ([]*grant, error) {
	rolesRaw, err := blackstart.ContextInputAs[[]string](mctx, inputRole, true)
	if err != nil {
		return nil, err
	}
	roles, err := normalizeRequiredStringList(rolesRaw, inputRole)
	if err != nil {
		return nil, err
	}

	permissionsRaw, err := blackstart.ContextInputAs[[]string](mctx, inputPermission, true)
	if err != nil {
		return nil, err
	}
	permissions, err := normalizeRequiredStringList(permissionsRaw, inputPermission)
	if err != nil {
		return nil, err
	}

	var schemas []string
	if values, present, schemaErr := readOptionalStringListInput(mctx, inputSchema); schemaErr != nil {
		return nil, fmt.Errorf("input %q is invalid: %w", inputSchema, schemaErr)
	} else if present {
		schemas = normalizeOptionalStringList(values)
	} else {
		schemas = []string{""}
	}

	var resources []string
	if values, present, resourceErr := readOptionalStringListInput(mctx, inputResource); resourceErr != nil {
		return nil, fmt.Errorf("input %q is invalid: %w", inputResource, resourceErr)
	} else if present {
		resources = normalizeOptionalStringList(values)
	} else {
		resources = []string{""}
	}

	scopeValue, err := blackstart.ContextInputAs[string](mctx, inputScope, false)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(scopeValue) == "" {
		scopeValue = "instance"
	}
	normalizedScope, err := stringToScope(scopeValue)
	if err != nil {
		return nil, err
	}
	schemas, resources, err = normalizeScopeTargets(normalizedScope, schemas, resources)
	if err != nil {
		return nil, err
	}

	targets := make([]*grant, 0, len(roles)*len(permissions)*len(schemas)*len(resources))
	for _, roleName := range roles {
		for _, permission := range permissions {
			normalizedPermission := normalizeGrantPermissionToken(normalizedScope, permission)
			if validationErr := validateGrantPrincipalAndPermission(
				normalizedScope, roleName, permission,
			); validationErr != nil {
				return nil, validationErr
			}
			for _, schema := range schemas {
				if schema != "" {
					if schemaErr := validatePostgresQuotedIdentifier(schema); schemaErr != nil {
						return nil, schemaErr
					}
				}
				for _, resource := range resources {
					if resource != "" {
						if resourceErr := validatePostgresQuotedIdentifier(resource); resourceErr != nil {
							return nil, resourceErr
						}
					}
					targets = append(
						targets, &grant{
							Role:       roleName,
							Permission: normalizedPermission,
							Schema:     schema,
							Resource:   resource,
							Scope:      string(normalizedScope),
						},
					)
				}
			}
		}
	}
	return targets, nil
}

type grantModule struct {
	db *sql.DB
}

func (g *grantModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   "postgres_grant",
		Name: "PostgreSQL grant",
		Description: "Ensures that a Postgres role has the specified Permission on a resource.\n\n" +
			"If multiple values are provided for `role`, `permission`, `schema`, or `resource`, " +
			"Blackstart expands all possible combinations of the Operation and applies them all.",
		Requirements: []string{
			"A valid Postgres `connection` input must be provided.",
			"The database user of the `connection` must be a member of a role that has `ADMIN OPTION` on the target roles.",
			"Target roles/users and target resources must exist for the selected `scope`.",
			"For `TABLE` scope, both schema and table must exist and be addressable by the user.",
		},
		Inputs: map[string]blackstart.InputValue{
			inputConnection: {
				Description: "database connection to the managed Postgres instance.",
				Type:        reflect.TypeFor[*sql.DB](),
				Required:    true,
			},
			inputRole: {
				Description: "Role(s) or username(s) that will have the grant assigned.",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    true,
			},
			inputPermission: {
				Description: "Permission(s) or role membership(s) to be assigned to the role(s). Depending on the resource scope, the valid permissions may vary.",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    true,
			},
			inputSchema: {
				Description: "Schema(s) where the permission is to be applied.",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    false,
			},
			inputResource: {
				Description: "Resource(s) where the permission(s) are to be applied. This might be a database name, table name, or schema name.",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    false,
			},
			inputScope: {
				Description: "Scope of the resource where the permission is to be applied. This might be a database, table, or schema.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
		},
		Outputs: map[string]blackstart.OutputValue{},
		Examples: map[string]string{
			"Grant role membership": `id: grant-role-membership
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: my-user
  permission: my-other-role`,
			"Grant schema usage": `id: grant-schema-usage
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: my-user
  permission: USAGE
  scope: SCHEMA
  resource: my-schema`,
			"Grant role membership at the instance level": `id: grant-instance-role-membership
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: my-user
  permission: app_readers
  scope: INSTANCE`,
			"Grant table permissions": `id: grant-orders-table-permissions
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: reporting_user
  permission:
    - SELECT
    - UPDATE
  scope: TABLE
  schema: public
  resource: orders`,
			"Grant schema permission to multiple roles": `id: grant-schema-usage-to-team
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role:
    - app_user
    - reporting_user
  permission: USAGE
  scope: SCHEMA
  resource: analytics`,
			"Grant multiple schema permissions for one user": `id: grant-user-schema-permissions
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission:
    - USAGE
    - CREATE
  scope: SCHEMA
  resource: app_data`,
			"Grant across multiple resources": `id: grant-multi-resource-permissions
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role:
    - app_user
    - reporting_user
  permission:
    - SELECT
    - UPDATE
  scope: TABLE
  schema:
    - public
    - analytics
  resource:
    - orders
    - invoices`,
		},
	}
}

func (g *grantModule) Validate(op blackstart.Operation) error {
	for _, p := range requiredGrantParameters {
		if _, ok := op.Inputs[p]; !ok {
			return fmt.Errorf("missing required parameter: %s", p)
		}
	}

	if connInput, ok := op.Inputs[inputConnection]; ok && connInput.IsStatic() && connInput.Any() == nil {
		return fmt.Errorf("missing required parameter: %s", inputConnection)
	}

	scopeValue := "instance"
	if scopeInput, ok := op.Inputs[inputScope]; ok && scopeInput.IsStatic() {
		parsedScope, err := blackstart.InputAs[string](scopeInput, false)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputScope, err)
		}
		if strings.TrimSpace(parsedScope) != "" {
			scopeValue = parsedScope
		}
	}
	grantScope, err := stringToScope(scopeValue)
	if err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", inputScope, err)
	}

	rolesInput := op.Inputs[inputRole]
	if rolesInput.IsStatic() {
		roles, rolesErr := blackstart.InputAs[[]string](rolesInput, true)
		if rolesErr != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputRole, rolesErr)
		}
		for _, roleName := range roles {
			if validationErr := validateGrantRole(roleName); validationErr != nil {
				return fmt.Errorf("parameter %s is invalid: %w", inputRole, validationErr)
			}
		}
	}

	permissionsInput := op.Inputs[inputPermission]
	if permissionsInput.IsStatic() {
		permissions, permissionErr := blackstart.InputAs[[]string](permissionsInput, true)
		if permissionErr != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputPermission, permissionErr)
		}
		for _, permission := range permissions {
			if validationErr := validateGrantPermission(grantScope, permission); validationErr != nil {
				return fmt.Errorf("parameter %s is invalid: %w", inputPermission, validationErr)
			}
		}
	}

	for _, p := range []string{inputSchema, inputResource} {
		o, ok := op.Inputs[p]
		if !ok || !o.IsStatic() {
			continue
		}
		values, valuesErr := blackstart.InputAs[[]string](o, false)
		if valuesErr != nil {
			return fmt.Errorf("parameter %s is invalid: %w", p, valuesErr)
		}
		for _, v := range values {
			if strings.TrimSpace(v) == "" {
				continue
			}
			if idErr := validatePostgresQuotedIdentifier(v); idErr != nil {
				return fmt.Errorf("parameter %s is invalid: %w", p, idErr)
			}
		}
	}

	return nil
}

func (g *grantModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	if err := g.setup(ctx); err != nil {
		return false, err
	}

	targets, err := expandGrantsFromContext(ctx)
	if err != nil {
		return false, err
	}

	for _, target := range targets {
		query, queryParams, queryErr := getGrantExistsQuery(target)
		if queryErr != nil {
			return false, fmt.Errorf("error getting grant query: %w", queryErr)
		}

		var exists bool
		err = g.db.QueryRowContext(ctx, query, queryParams...).Scan(&exists)
		if err != nil {
			return false, fmt.Errorf("error checking grant: %w", err)
		}

		if ctx.DoesNotExist() {
			if exists {
				return false, nil
			}
			continue
		}
		if !exists {
			return false, nil
		}
	}

	return true, nil
}

func (g *grantModule) Set(ctx blackstart.ModuleContext) error {
	if err := g.setup(ctx); err != nil {
		return err
	}

	targets, err := expandGrantsFromContext(ctx)
	if err != nil {
		return err
	}

	for _, target := range targets {
		if ctx.DoesNotExist() {
			query, queryParams, revokeErr := getGrantRevokeQuery(target)
			if revokeErr != nil {
				return fmt.Errorf("error getting revoke query: %w", revokeErr)
			}
			_, err = g.db.ExecContext(ctx, query, queryParams...)
			if err != nil {
				return fmt.Errorf("error revoking grant: %w", err)
			}
			continue
		}

		query, queryParams, queryErr := getGrantSetQuery(target)
		if queryErr != nil {
			return fmt.Errorf("error getting grant query: %w", queryErr)
		}

		_, err = g.db.ExecContext(ctx, query, queryParams...)
		if err != nil {
			return fmt.Errorf("error setting grant: %w", err)
		}
	}

	return nil
}

// setup initializes the grantModule by extracting the database connection from the module context.
func (g *grantModule) setup(ctx blackstart.ModuleContext) error {
	conn, err := blackstart.ContextInputAs[*sql.DB](ctx, inputConnection, true)
	if err != nil {
		return err
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
		queryParams := []interface{}{normalizeGrantPermissionToken(grantScope, target.Permission), target.Role}
		return getGrantInstanceQuery, queryParams, nil
	case scopes.database:
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			queryParams := []interface{}{target.Role, target.Resource}
			return getGrantDatabaseAllQuery, queryParams, nil
		}
		queryParams := []interface{}{target.Role, normalizeGrantPermissionToken(
			grantScope, target.Permission,
		), target.Resource}
		return getGrantDatabaseQuery, queryParams, nil
	case scopes.schema:
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			queryParams := []interface{}{target.Role, target.Resource}
			return getGrantSchemaAllQuery, queryParams, nil
		}
		queryParams := []interface{}{normalizeGrantPermissionToken(
			grantScope, target.Permission,
		), target.Role, target.Resource}
		return getGrantSchemaQuery, queryParams, nil
	case scopes.table:
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			queryParams := []interface{}{target.Role, target.Schema, target.Resource}
			return getGrantTableAllQuery, queryParams, nil
		}
		queryParams := []interface{}{normalizeGrantPermissionToken(
			grantScope, target.Permission,
		), target.Role, target.Resource, target.Schema}
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

	normalizedTarget := *target
	normalizedTarget.Permission = normalizeGrantPermissionToken(grantScope, target.Permission)
	perm := normalizedTarget.Permission
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
		if !slices.Contains(grantTablePermissions, perm) {
			return "", nil, fmt.Errorf("invalid table Permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantTable").Parse(setGrantTableTemplate)
	default:
		return "", nil, fmt.Errorf("no query for Scope: %s", target.Scope)
	}

	if err != nil {
		return "", nil, err
	}

	var queryBuffer bytes.Buffer
	err = tmpl.Execute(&queryBuffer, &normalizedTarget)
	if err != nil {
		return "", nil, err
	}

	return queryBuffer.String(), nil, nil
}

// getGrantRevokeQuery constructs the SQL query to revoke a grant based on the target grant object's
// Scope. It returns the query string and query parameters.
//
// goland:noinspection SqlNoDataSourceInspection
func getGrantRevokeQuery(target *grant) (string, []interface{}, error) {
	grantScope, err := stringToScope(target.Scope)
	if err != nil {
		return "", nil, err
	}

	normalizedTarget := *target
	normalizedTarget.Permission = normalizeGrantPermissionToken(grantScope, target.Permission)
	perm := normalizedTarget.Permission
	var tmpl *template.Template
	switch grantScope {
	case scopes.instance:
		tmpl, err = template.New("revokeGrantInstance").Parse(setRevokeInstanceTemplate)
	case scopes.database:
		if !slices.Contains(grantDatabasePermissions, perm) {
			return "", nil, fmt.Errorf("invalid database Permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantDatabase").Parse(setRevokeDatabaseTemplate)
	case scopes.schema:
		if !slices.Contains(grantSchemaPermissions, perm) {
			return "", nil, fmt.Errorf("invalid Schema Permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantSchema").Parse(setRevokeSchemaTemplate)
	case scopes.table:
		if !slices.Contains(grantTablePermissions, perm) {
			return "", nil, fmt.Errorf("invalid table Permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantTable").Parse(setRevokeTableTemplate)
	default:
		return "", nil, fmt.Errorf("no revoke query for Scope: %s", target.Scope)
	}

	if err != nil {
		return "", nil, err
	}

	var queryBuffer bytes.Buffer
	err = tmpl.Execute(&queryBuffer, &normalizedTarget)
	if err != nil {
		return "", nil, err
	}

	return queryBuffer.String(), nil, nil
}
