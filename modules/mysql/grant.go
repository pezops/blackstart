package mysql

import (
	"database/sql"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

func init() {
	blackstart.RegisterModule("mysql_grant", NewMySQLGrant)
}

// scope is the resource-level scope where a grant is applied.
type scope string

// scopes contains the valid scopes for a grant operation.
var scopes = struct {
	database scope
	table    scope
}{
	database: "DATABASE",
	table:    "TABLE",
}

var _ blackstart.Module = &grantModule{}
var requiredGrantParameters = []string{inputRole, inputPermission, inputConnection}
var grantDatabasePermissions = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "REFERENCES", "INDEX", "ALTER",
	"CREATE VIEW", "SHOW VIEW", "TRIGGER", "EVENT", "EXECUTE", "ALL",
}
var grantTablePermissions = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "CREATE", "DROP", "REFERENCES", "INDEX", "ALTER",
	"CREATE VIEW", "SHOW VIEW", "TRIGGER", "ALL",
}

// NewMySQLGrant creates a new instance of the MySQL grant module.
func NewMySQLGrant() blackstart.Module {
	return &grantModule{}
}

// grant contains the information for a MySQL grant operation.
type grant struct {
	// Role is the MySQL account or role that will have the grant assigned.
	Role string
	// Permission is the privilege to be assigned.
	Permission string
	// Schema is the database name for TABLE scope.
	Schema string
	// Resource is the database name for DATABASE scope or table name for TABLE scope.
	Resource string
	// Scope is the target type where the Permission is applied.
	Scope scope
	// All indicates the grant should target all tables in Schema.
	All bool
	// WithGrantOption requests WITH GRANT OPTION.
	WithGrantOption bool
}

// grantModule ensures MySQL grants exist or do not exist.
type grantModule struct {
	db *sql.DB
}

// Info returns metadata describing the MySQL grant module.
func (g *grantModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   "mysql_grant",
		Name: "MySQL grant",
		Description: util.CleanString(
			`
Ensures that MySQL accounts or roles have the specified permissions on databases or tables.

If multiple values are provided for '''role''', '''permission''', '''schema''', or '''resource''',
Blackstart expands all possible combinations of the Operation and applies them all.
`,
		),
		Requirements: []string{
			"A valid MySQL `connection` input must be provided.",
			"The database user of the `connection` must have sufficient privileges to apply the requested grants.",
			"Target accounts/roles and resources must exist for the selected `scope`.",
		},
		Inputs: map[string]blackstart.InputValue{
			inputConnection: {
				Description: "Database connection to the managed MySQL instance.",
				Type:        reflect.TypeFor[*sql.DB](),
				Required:    true,
			},
			inputRole: {
				Description: "Account(s) or role(s) that will have the grant assigned. Host defaults to `%` when omitted.",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    true,
			},
			inputPermission: {
				Description: "Permission(s) to assign to the role(s).",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    true,
			},
			inputSchema: {
				Description: "Database name for `TABLE` scope.",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    false,
			},
			inputResource: {
				Description: "Database name for `DATABASE` scope or table name for `TABLE` scope.",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    false,
			},
			inputScope: {
				Description: "Scope of the resource where the permission is applied. Supported values: `DATABASE`, `TABLE`.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "DATABASE",
			},
			inputAll: {
				Description: "Apply table permissions to all tables in the database named by `schema`.",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
				Default:     false,
			},
			inputWithGrantOption: {
				Description: "Request `WITH GRANT OPTION`.",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
				Default:     false,
			},
		},
		Outputs: map[string]blackstart.OutputValue{},
		Examples: map[string]string{
			"Grant database permissions": `id: grant-app-db-select
module: mysql_grant
inputs:
  connection:
    fromDependency:
      id: connect-db
      output: connection
  role: app_user
  permission: SELECT
  scope: DATABASE
  resource: app`,
			"Grant table permissions": `id: grant-orders-select
module: mysql_grant
inputs:
  connection:
    fromDependency:
      id: connect-db
      output: connection
  role: reporting_user
  permission: SELECT
  scope: TABLE
  schema: app
  resource: orders`,
		},
	}
}

// Validate checks whether an operation contains valid MySQL grant inputs.
func (g *grantModule) Validate(op blackstart.Operation) error {
	for _, p := range requiredGrantParameters {
		if _, ok := op.Inputs[p]; !ok {
			return fmt.Errorf("missing required parameter: %s", p)
		}
	}

	if connInput, ok := op.Inputs[inputConnection]; ok && connInput.IsStatic() && connInput.Any() == nil {
		return fmt.Errorf("missing required parameter: %s", inputConnection)
	}

	grantScope, err := staticGrantScope(op)
	if err != nil {
		return err
	}

	all, err := staticBoolInput(op, inputAll)
	if err != nil {
		return err
	}

	if all && grantScope != scopes.table {
		return fmt.Errorf("parameter %s is invalid: only supported when scope is TABLE", inputAll)
	}

	if err = validateStaticStringListInput(op, inputRole, true, validateMySQLRole); err != nil {
		return err
	}

	if err = validateStaticStringListInput(
		op, inputPermission, true, func(value string) error {
			return validateGrantPermission(grantScope, value)
		},
	); err != nil {
		return err
	}

	if err = validateStaticStringListInput(op, inputSchema, false, validateGrantIdentifier); err != nil {
		return err
	}

	if err = validateStaticStringListInput(op, inputResource, false, validateGrantIdentifier); err != nil {
		return err
	}

	if all {
		if resourceInput, ok := op.Inputs[inputResource]; ok && resourceInput.IsStatic() {
			resources, resourceErr := blackstart.InputAs[[]string](resourceInput, false)

			if resourceErr != nil {
				return fmt.Errorf("parameter %s is invalid: %w", inputResource, resourceErr)
			}

			for _, resource := range resources {
				if strings.TrimSpace(resource) != "" {
					return fmt.Errorf("parameter %s is invalid: must be empty when %s is true", inputResource, inputAll)
				}
			}
		}
	}

	return nil
}

// Check reports whether all target grants are in the requested state.
func (g *grantModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	if err := g.setup(ctx); err != nil {
		return false, err
	}

	targets, err := expandGrantsFromContext(ctx)
	if err != nil {
		return false, err
	}

	for _, target := range targets {
		existsQueries, queryErr := getGrantExistsQueries(target)
		if queryErr != nil {
			return false, fmt.Errorf("error getting grant query: %w", queryErr)
		}

		for _, existsQuery := range existsQueries {
			var exists bool
			if err = g.db.QueryRowContext(ctx, existsQuery.query, existsQuery.params...).Scan(&exists); err != nil {
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
	}

	return true, nil
}

// Set applies or revokes all target grants.
func (g *grantModule) Set(ctx blackstart.ModuleContext) error {
	if err := g.setup(ctx); err != nil {
		return err
	}

	targets, err := expandGrantsFromContext(ctx)
	if err != nil {
		return err
	}

	for _, target := range targets {
		var stmt string
		if ctx.DoesNotExist() {
			stmt, err = renderRevokeSQL(target)
		} else {
			stmt, err = renderGrantSQL(target)
		}

		if err != nil {
			return err
		}

		if _, err = g.db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("error applying grant: %w", err)
		}
	}

	return nil
}

// setup initializes the grant module from context.
func (g *grantModule) setup(ctx blackstart.ModuleContext) error {
	db, err := blackstart.ContextInputAs[*sql.DB](ctx, inputConnection, true)
	if err != nil {
		return err
	}

	g.db = db

	return nil
}

// expandGrantsFromContext expands single or list-valued inputs into all grant combinations.
func expandGrantsFromContext(ctx blackstart.ModuleContext) ([]*grant, error) {
	roles, err := contextStringList(ctx, inputRole, true)
	if err != nil {
		return nil, err
	}

	permissions, err := contextStringList(ctx, inputPermission, true)
	if err != nil {
		return nil, err
	}

	schemas, err := contextStringList(ctx, inputSchema, false)
	if err != nil {
		return nil, err
	}

	resources, err := contextStringList(ctx, inputResource, false)
	if err != nil {
		return nil, err
	}

	scopeValue, err := blackstart.ContextInputAs[string](ctx, inputScope, false)
	if err != nil {
		return nil, err
	}

	grantScope, err := stringToScope(scopeValue)
	if err != nil {
		return nil, err
	}

	all, err := blackstart.ContextInputAs[bool](ctx, inputAll, false)
	if err != nil {
		return nil, err
	}

	withGrantOption, err := blackstart.ContextInputAs[bool](ctx, inputWithGrantOption, false)
	if err != nil {
		return nil, err
	}

	schemas, resources, err = normalizeScopeTargets(grantScope, schemas, resources, all)
	if err != nil {
		return nil, err
	}

	targets := make([]*grant, 0, len(roles)*len(permissions)*len(schemas)*len(resources))
	for _, role := range roles {
		for _, permission := range permissions {
			normalizedPermission, permissionErr := normalizeGrantPermission(grantScope, permission)
			if permissionErr != nil {
				return nil, permissionErr
			}

			for _, schema := range schemas {
				for _, resource := range resources {
					targets = append(
						targets, &grant{
							Role:            role,
							Permission:      normalizedPermission,
							Schema:          schema,
							Resource:        resource,
							Scope:           grantScope,
							All:             all,
							WithGrantOption: withGrantOption,
						},
					)
				}
			}
		}
	}

	return targets, nil
}

// contextStringList returns a normalized list-valued context input.
func contextStringList(ctx blackstart.ModuleContext, key string, required bool) ([]string, error) {
	values, err := blackstart.ContextInputAs[[]string](ctx, key, required)
	if err != nil {
		return nil, err
	}

	if required {
		return normalizeRequiredStringList(values, key)
	}

	return normalizeOptionalStringList(values), nil
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

// normalizeOptionalStringList trims optional list values and collapses empty input to one marker.
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

// normalizeScopeTargets validates and normalizes schema/resource inputs for a scope.
func normalizeScopeTargets(
	grantScope scope, schemas []string, resources []string, all bool,
) ([]string, []string, error) {
	switch grantScope {
	case scopes.database:
		if len(resources) == 1 && resources[0] == "" {
			return nil, nil, fmt.Errorf("input %q must be provided when scope is DATABASE", inputResource)
		}
		return []string{""}, resources, nil
	case scopes.table:
		if len(schemas) == 1 && schemas[0] == "" {
			return nil, nil, fmt.Errorf("input %q must be provided when scope is TABLE", inputSchema)
		}

		if all {
			return schemas, []string{""}, nil
		}

		if len(resources) == 1 && resources[0] == "" {
			return nil, nil, fmt.Errorf("input %q must be provided when scope is TABLE", inputResource)
		}
		return schemas, resources, nil
	default:
		return nil, nil, fmt.Errorf("unsupported scope: %s", grantScope)
	}
}

// stringToScope converts a scope input string into a scope constant.
func stringToScope(input string) (scope, error) {
	normalized := strings.ToUpper(strings.TrimSpace(input))
	if normalized == "" {
		normalized = string(scopes.database)
	}

	switch scope(normalized) {
	case scopes.database:
		return scopes.database, nil
	case scopes.table:
		return scopes.table, nil
	default:
		return "", fmt.Errorf("invalid scope: %s", input)
	}
}

// staticGrantScope returns the static grant scope for validation.
func staticGrantScope(op blackstart.Operation) (scope, error) {
	scopeValue := ""
	if scopeInput, ok := op.Inputs[inputScope]; ok && scopeInput.IsStatic() {
		parsedScope, err := blackstart.InputAs[string](scopeInput, false)
		if err != nil {
			return "", fmt.Errorf("parameter %s is invalid: %w", inputScope, err)
		}
		scopeValue = parsedScope
	}

	grantScope, err := stringToScope(scopeValue)
	if err != nil {
		return "", fmt.Errorf("parameter %s is invalid: %w", inputScope, err)
	}

	return grantScope, nil
}

// staticBoolInput returns a static bool input when configured.
func staticBoolInput(op blackstart.Operation, key string) (bool, error) {
	input, ok := op.Inputs[key]
	if !ok || !input.IsStatic() {
		return false, nil
	}

	value, err := blackstart.InputAs[bool](input, false)
	if err != nil {
		return false, fmt.Errorf("parameter %s is invalid: %w", key, err)
	}

	return value, nil
}

// validateStaticStringListInput validates a static string or string-list input.
func validateStaticStringListInput(
	op blackstart.Operation, key string, required bool, validate func(string) error,
) error {
	input, ok := op.Inputs[key]
	if !ok || !input.IsStatic() {
		return nil
	}

	values, err := blackstart.InputAs[[]string](input, required)
	if err != nil {
		return fmt.Errorf("parameter %s is invalid: %w", key, err)
	}

	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			if required {
				return fmt.Errorf("parameter %s is invalid: value cannot be empty", key)
			}
			continue
		}

		if err = validate(trimmed); err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", key, err)
		}
	}
	return nil
}

// validateMySQLRole validates a MySQL account or role input.
func validateMySQLRole(value string) error {
	_, err := parseMySQLAccount(value)
	return err
}

// validateGrantIdentifier validates a database or table identifier.
func validateGrantIdentifier(value string) error {
	_, err := quoteIdentifier(value)
	return err
}

// validateGrantPermission validates a permission for the selected scope.
func validateGrantPermission(grantScope scope, permission string) error {
	_, err := normalizeGrantPermission(grantScope, permission)
	return err
}

// normalizeGrantPermission returns the normalized permission token.
func normalizeGrantPermission(grantScope scope, permission string) (string, error) {
	normalized := strings.ToUpper(strings.Join(strings.Fields(permission), " "))
	if normalized == "" {
		return "", fmt.Errorf("permission cannot be empty")
	}

	var allowed []string
	switch grantScope {
	case scopes.database:
		allowed = grantDatabasePermissions
	case scopes.table:
		allowed = grantTablePermissions
	default:
		return "", fmt.Errorf("unsupported scope: %s", grantScope)
	}

	if !slices.Contains(allowed, normalized) {
		return "", fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
	}

	return normalized, nil
}
