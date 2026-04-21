package postgres

import (
	"bytes"
	"database/sql"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"text/template"

	"github.com/lib/pq"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

func init() {
	blackstart.RegisterModule("postgres_grant", NewPostgresGrant)
}

// scope is the Resource-level scope where a grant is applied. This might be an instance, Schema, table,
// or database.
type scope string

// scopes contains the valid scopes for a grant operation that can be referenced in code.
var scopes = struct {
	instance      scope
	schema        scope
	table         scope
	sequence      scope
	function      scope
	procedure     scope
	routine       scope
	database      scope
	domain        scope
	fdw           scope
	foreignServer scope
	language      scope
	largeObject   scope
	parameter     scope
	tablespace    scope
	typ           scope
}{
	instance:      "INSTANCE",
	schema:        "SCHEMA",
	table:         "TABLE",
	sequence:      "SEQUENCE",
	function:      "FUNCTION",
	procedure:     "PROCEDURE",
	routine:       "ROUTINE",
	database:      "DATABASE",
	domain:        "DOMAIN",
	fdw:           "FDW",
	foreignServer: "FOREIGN_SERVER",
	language:      "LANGUAGE",
	largeObject:   "LARGE_OBJECT",
	parameter:     "PARAMETER",
	tablespace:    "TABLESPACE",
	typ:           "TYPE",
}

// scopesList contains the valid scopes for a grant operation that can be referenced in code.
var scopesList = []scope{
	scopes.instance,
	scopes.schema,
	scopes.table,
	scopes.sequence,
	scopes.function,
	scopes.procedure,
	scopes.routine,
	scopes.database,
	scopes.domain,
	scopes.fdw,
	scopes.foreignServer,
	scopes.language,
	scopes.largeObject,
	scopes.parameter,
	scopes.tablespace,
	scopes.typ,
}

var _ blackstart.Module = &grantModule{}
var requiredGrantParameters = []string{inputRole, inputPermission, inputConnection}
var grantSchemaPermissions = []string{"CREATE", "USAGE", "ALL"}
var grantDatabasePermissions = []string{"CREATE", "CONNECT", "TEMPORARY", "TEMP", "ALL"}
var grantTablePermissions = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN", "ALL",
}
var grantSequencePermissions = []string{"USAGE", "SELECT", "UPDATE", "ALL"}
var grantRoutinePermissions = []string{"EXECUTE", "ALL"}
var grantDomainPermissions = []string{"USAGE", "ALL"}
var grantFdwPermissions = []string{"USAGE", "ALL"}
var grantForeignServerPermissions = []string{"USAGE", "ALL"}
var grantLanguagePermissions = []string{"USAGE", "ALL"}
var grantLargeObjectPermissions = []string{"SELECT", "UPDATE", "ALL"}
var grantParameterPermissions = []string{"SET", "ALTER SYSTEM", "ALL"}
var grantTablespacePermissions = []string{"CREATE", "ALL"}
var grantTypePermissions = []string{"USAGE", "ALL"}
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
	// Schema is the name of a PostgreSQL database schema where the Permission is to be applied. Defaults to
	// the "public" schema.
	Schema string
	// Resource is the name of the database object for the Permission to be applied. This might be a database
	// name, table name, or schema name.
	Resource string
	// Scope is the type of Resource where the Permission is to be applied. This might be a database,
	// table, or schema.
	Scope string
	// All indicates the grant should target all resources in Schema for scopes that support it.
	All bool
	// WithGrantOption requests WITH GRANT OPTION where supported by the selected scope.
	WithGrantOption bool
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
	grantScope scope, schemas []string, resources []string, all bool,
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
		if all {
			if len(resources) > 1 || (len(resources) == 1 && resources[0] != "") {
				return nil, nil, fmt.Errorf(
					"input %q must be empty when %q is true", inputResource, inputAll,
				)
			}
			return schemas, []string{""}, nil
		}
		if len(resources) == 1 && resources[0] == "" {
			return nil, nil, fmt.Errorf("input %q must be provided when scope is TABLE", inputResource)
		}
		return schemas, resources, nil
	case scopes.sequence:
		// SEQUENCE grants target schema-qualified sequences. Set the default schema to public when omitted.
		if len(schemas) == 1 && schemas[0] == "" {
			schemas = []string{"public"}
		}
		if all {
			if len(resources) > 1 || (len(resources) == 1 && resources[0] != "") {
				return nil, nil, fmt.Errorf(
					"input %q must be empty when %q is true", inputResource, inputAll,
				)
			}
			return schemas, []string{""}, nil
		}
		if len(resources) == 1 && resources[0] == "" {
			return nil, nil, fmt.Errorf("input %q must be provided when scope is SEQUENCE", inputResource)
		}
		return schemas, resources, nil
	case scopes.function, scopes.procedure, scopes.routine:
		// Routine grants require schema-qualified resources, or schema-wide "all" mode.
		if len(schemas) == 1 && schemas[0] == "" {
			return nil, nil, fmt.Errorf("input %q must be provided when scope is %s", inputSchema, grantScope)
		}
		if all {
			if len(resources) > 1 || (len(resources) == 1 && resources[0] != "") {
				return nil, nil, fmt.Errorf(
					"input %q must be empty when %q is true", inputResource, inputAll,
				)
			}
			return schemas, []string{""}, nil
		}
		if len(resources) == 1 && resources[0] == "" {
			return nil, nil, fmt.Errorf("input %q must be provided when scope is %s", inputResource, grantScope)
		}
		return schemas, resources, nil
	case scopes.domain, scopes.fdw, scopes.foreignServer, scopes.language, scopes.largeObject, scopes.parameter, scopes.tablespace, scopes.typ:
		// Object scopes outside schema-qualified resources always target inputResource directly.
		if len(resources) == 1 && resources[0] == "" {
			return nil, nil, fmt.Errorf("input %q must be provided when scope is %s", inputResource, grantScope)
		}
		if len(schemas) > 1 || (len(schemas) == 1 && schemas[0] != "") {
			return nil, nil, fmt.Errorf("input %q is not supported when scope is %s", inputSchema, grantScope)
		}
		return []string{""}, resources, nil
	default:
		return nil, nil, fmt.Errorf("unsupported scope: %s", grantScope)
	}
}

// supportsWithGrantOption reports whether GRANT ... WITH GRANT OPTION is supported for the scope.
func supportsWithGrantOption(grantScope scope) bool {
	return grantScope != scopes.instance
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
	case scopes.sequence:
		if !slices.Contains(grantSequencePermissions, perm) {
			return fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
		}
	case scopes.function, scopes.procedure, scopes.routine:
		if !slices.Contains(grantRoutinePermissions, perm) {
			return fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
		}
	case scopes.domain:
		if !slices.Contains(grantDomainPermissions, perm) {
			return fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
		}
	case scopes.fdw:
		if !slices.Contains(grantFdwPermissions, perm) {
			return fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
		}
	case scopes.foreignServer:
		if !slices.Contains(grantForeignServerPermissions, perm) {
			return fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
		}
	case scopes.language:
		if !slices.Contains(grantLanguagePermissions, perm) {
			return fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
		}
	case scopes.largeObject:
		if !slices.Contains(grantLargeObjectPermissions, perm) {
			return fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
		}
	case scopes.parameter:
		if !slices.Contains(grantParameterPermissions, perm) {
			return fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
		}
	case scopes.tablespace:
		if !slices.Contains(grantTablespacePermissions, perm) {
			return fmt.Errorf("invalid permission %q for scope %s", permission, grantScope)
		}
	case scopes.typ:
		if !slices.Contains(grantTypePermissions, perm) {
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

// checkPermissionToken formats permission tokens for has_*_privilege checks.
func checkPermissionToken(permission string, withGrantOption bool) string {
	if withGrantOption {
		return permission + " WITH GRANT OPTION"
	}
	return permission
}

// applyWithGrantOptionClause appends WITH GRANT OPTION to rendered GRANT statements.
func applyWithGrantOptionClause(query string, enabled bool) string {
	if !enabled {
		return query
	}
	trimmed := strings.TrimSpace(query)
	trimmed = strings.TrimSuffix(trimmed, ";")
	return trimmed + " WITH GRANT OPTION;"
}

// validateGrantResource validates object resource names for the target grant scope.
func validateGrantResource(grantScope scope, resource string) error {
	if strings.TrimSpace(resource) == "" {
		return nil
	}
	if grantScope == scopes.largeObject {
		loid, err := strconv.ParseUint(resource, 10, 32)
		if err != nil || loid == 0 {
			return fmt.Errorf("large object resource must be a positive integer loid")
		}
		return nil
	}
	return validatePostgresQuotedIdentifier(resource)
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
	all, err := blackstart.ContextInputAs[bool](mctx, inputAll, false)
	if err != nil {
		return nil, err
	}
	withGrantOption, err := blackstart.ContextInputAs[bool](mctx, inputWithGrantOption, false)
	if err != nil {
		return nil, err
	}
	if all &&
		normalizedScope != scopes.table &&
		normalizedScope != scopes.sequence &&
		normalizedScope != scopes.function &&
		normalizedScope != scopes.procedure &&
		normalizedScope != scopes.routine {
		return nil, fmt.Errorf(
			"input %q is only supported when scope is TABLE, SEQUENCE, FUNCTION, PROCEDURE, or ROUTINE",
			inputAll,
		)
	}
	schemas, resources, err = normalizeScopeTargets(normalizedScope, schemas, resources, all)
	if err != nil {
		return nil, err
	}
	if withGrantOption && !supportsWithGrantOption(normalizedScope) {
		return nil, fmt.Errorf(
			"input %q is not supported when scope is %s",
			inputWithGrantOption, normalizedScope,
		)
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
						if resourceErr := validateGrantResource(normalizedScope, resource); resourceErr != nil {
							return nil, resourceErr
						}
					}
					targets = append(
						targets, &grant{
							Role:            roleName,
							Permission:      normalizedPermission,
							Schema:          schema,
							Resource:        resource,
							Scope:           string(normalizedScope),
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

type grantModule struct {
	db *sql.DB
}

func (g *grantModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   "postgres_grant",
		Name: "PostgreSQL grant",
		Description: util.CleanString(
			`
Ensures that PostgreSQL roles have the specified permissions on resources. The scope specifies the 
type of resources where the permissions will be applied.

If multiple values are provided for '''role''', '''permission''', '''schema''', or '''resource''', 
Blackstart expands all possible combinations of the Operation and applies them all.

The permissions allowed vary by scope. See the [PostgreSQL GRANT documentation](https://www.postgresql.org/docs/current/sql-grant.html) 
for details on valid permissions for each scope.
`,
		),
		Requirements: []string{
			"A valid PostgreSQL `connection` input must be provided.",
			"The database user of the `connection` must have sufficient privileges to apply the requested grants.",
			"Target roles/users and target resources must exist for the selected `scope`.",
			"For `TABLE` and `SEQUENCE` scopes, both schema and resource must exist and be addressable by the user.",
			"For `FUNCTION`, `PROCEDURE`, and `ROUTINE` scopes, `schema` must be provided and `resource` must be a routine signature that includes argument types unless `all` is true.",
			"`LARGE_OBJECT` scope requires `resource` to be a numeric large object OID (`loid`).",
			"`PARAMETER` scope requires a PostgreSQL version that supports parameter privileges (`GRANT ... ON PARAMETER ...`) and `has_parameter_privilege`.",
		},
		Inputs: map[string]blackstart.InputValue{
			inputConnection: {
				Description: "database connection to the managed PostgreSQL instance.",
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
				Description: "Resource(s) where the permission(s) are applied. For `FUNCTION`, `PROCEDURE`, and `ROUTINE` scopes, provide a routine signature with argument types.",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    false,
			},
			inputScope: {
				Description: "Scope of the resource where the permission is to be applied. Supported values: `INSTANCE`, `DATABASE`, `SCHEMA`, `TABLE`, `SEQUENCE`, `FUNCTION`, `PROCEDURE`, `ROUTINE`, `DOMAIN`, `FDW`, `FOREIGN_SERVER`, `LANGUAGE`, `LARGE_OBJECT`, `PARAMETER`, `TABLESPACE`, `TYPE`.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
			inputAll: {
				Description: "Apply permissions to all resources of the scope (if supported) in the schema. When set, the resource input must be empty.",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
				Default:     false,
			},
			inputWithGrantOption: {
				Description: "Request `WITH GRANT OPTION` for supported scopes. Not supported for `INSTANCE` scope.",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
				Default:     false,
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
			"Grant SELECT on all tables in a schema": `id: grant-select-all-tables-in-public
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: reporting_user
  permission: SELECT
  scope: TABLE
  schema: public
  all: true`,
			"Grant USAGE on all sequences in a schema": `id: grant-usage-all-sequences-in-public
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: USAGE
  scope: SEQUENCE
  schema: public
  all: true`,
			"Grant EXECUTE on a function": `id: grant-execute-function
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: EXECUTE
  scope: FUNCTION
  schema: public
  resource: do_work(integer)`,
			"Grant EXECUTE on all procedures in schema": `id: grant-execute-all-procedures
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: EXECUTE
  scope: PROCEDURE
  schema: public
  all: true`,
			"Grant EXECUTE on all routines in schema": `id: grant-execute-all-routines
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: EXECUTE
  scope: ROUTINE
  schema: public
  all: true`,
			"Grant SELECT with grant option on a table": `id: grant-select-with-grant-option
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: SELECT
  scope: TABLE
  schema: public
  resource: orders
  with_grant_option: true`,
			"Grant USAGE on a type": `id: grant-usage-on-type
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: USAGE
  scope: TYPE
  resource: status_type`,
			"Grant SET on a configuration parameter": `id: grant-set-on-parameter
module: postgres_grant
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: app_user
  permission: SET
  scope: PARAMETER
  resource: work_mem`,
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
	all := false
	if allInput, ok := op.Inputs[inputAll]; ok && allInput.IsStatic() {
		all, err = blackstart.InputAs[bool](allInput, false)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputAll, err)
		}
	}
	withGrantOption := false
	if withGrantOptionInput, ok := op.Inputs[inputWithGrantOption]; ok && withGrantOptionInput.IsStatic() {
		withGrantOption, err = blackstart.InputAs[bool](withGrantOptionInput, false)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputWithGrantOption, err)
		}
	}
	if withGrantOption && !supportsWithGrantOption(grantScope) {
		return fmt.Errorf(
			"parameter %s is invalid: not supported when scope is %s",
			inputWithGrantOption, grantScope,
		)
	}
	if all &&
		grantScope != scopes.table &&
		grantScope != scopes.sequence &&
		grantScope != scopes.function &&
		grantScope != scopes.procedure &&
		grantScope != scopes.routine {
		return fmt.Errorf(
			"parameter %s is invalid: only supported when scope is TABLE, SEQUENCE, FUNCTION, PROCEDURE, or ROUTINE",
			inputAll,
		)
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
			if p == inputSchema && (grantScope == scopes.domain || grantScope == scopes.fdw || grantScope == scopes.foreignServer || grantScope == scopes.language || grantScope == scopes.largeObject || grantScope == scopes.parameter || grantScope == scopes.tablespace || grantScope == scopes.typ) {
				return fmt.Errorf("parameter %s is invalid: not supported when scope is %s", p, grantScope)
			}
			var idErr error
			if p == inputSchema {
				idErr = validatePostgresQuotedIdentifier(v)
			} else {
				idErr = validateGrantResource(grantScope, v)
			}
			if idErr != nil {
				return fmt.Errorf("parameter %s is invalid: %w", p, idErr)
			}
			if p == inputResource && all {
				return fmt.Errorf("parameter %s is invalid: must be empty when %s is true", p, inputAll)
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
		existsQueries, queryErr := getGrantExistsQueries(target)
		if queryErr != nil {
			return false, fmt.Errorf("error getting grant query: %w", queryErr)
		}

		exists := true
		for _, existsQuery := range existsQueries {
			var queryExists bool
			err = g.db.QueryRowContext(ctx, existsQuery.query, existsQuery.params...).Scan(&queryExists)
			if err != nil {
				if isPQInvalidParameterValueError(err) {
					// Ignore unsupported privilege checks (for example, newer privileges on
					// older server versions) and continue evaluating remaining checks.
					continue
				}
				return false, fmt.Errorf("error checking grant: %w", err)
			}
			if !queryExists {
				exists = false
				break
			}
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

// isPQInvalidParameterValueError returns true for PostgreSQL SQLSTATE 22023.
func isPQInvalidParameterValueError(err error) bool {
	var pqErr *pq.Error
	if !errors.As(err, &pqErr) {
		return false
	}
	return string(pqErr.Code) == "22023"
}

type grantExistsQuery struct {
	query  string
	params []interface{}
}

// getGrantExistsQueries returns one or more check queries for a grant target.
// For ALL permissions, it expands to per-permission checks.
func getGrantExistsQueries(target *grant) ([]grantExistsQuery, error) {
	grantScope, err := stringToScope(target.Scope)
	if err != nil {
		return nil, err
	}
	normalizedPermission := normalizeGrantPermissionToken(grantScope, target.Permission)
	if normalizedPermission != "ALL" {
		q, p, queryErr := getGrantExistsQuery(target)
		if queryErr != nil {
			return nil, queryErr
		}
		return []grantExistsQuery{{query: q, params: p}}, nil
	}

	allPermissions := grantAllPermissions(grantScope)
	if len(allPermissions) == 0 {
		// Defensive fallback; INSTANCE scope with ALL is already invalid upstream.
		q, p, queryErr := getGrantExistsQuery(target)
		if queryErr != nil {
			return nil, queryErr
		}
		return []grantExistsQuery{{query: q, params: p}}, nil
	}

	queries := make([]grantExistsQuery, 0, len(allPermissions))
	for _, permission := range allPermissions {
		specific := *target
		specific.Permission = permission
		q, p, queryErr := getGrantExistsQuery(&specific)
		if queryErr != nil {
			return nil, queryErr
		}
		queries = append(queries, grantExistsQuery{query: q, params: p})
	}
	return queries, nil
}

// grantAllPermissions expands ALL for each scope into concrete permissions.
func grantAllPermissions(grantScope scope) []string {
	switch grantScope {
	case scopes.database:
		return []string{"CREATE", "CONNECT", "TEMPORARY"}
	case scopes.schema:
		return []string{"CREATE", "USAGE"}
	case scopes.table:
		return []string{"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN"}
	case scopes.sequence:
		return []string{"USAGE", "SELECT", "UPDATE"}
	case scopes.function, scopes.procedure, scopes.routine:
		return []string{"EXECUTE"}
	case scopes.domain, scopes.fdw, scopes.foreignServer, scopes.language, scopes.typ:
		return []string{"USAGE"}
	case scopes.largeObject:
		return []string{"SELECT", "UPDATE"}
	case scopes.parameter:
		return []string{"SET", "ALTER SYSTEM"}
	case scopes.tablespace:
		return []string{"CREATE"}
	default:
		return nil
	}
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
			queryParams := []interface{}{
				target.Role,
				target.Resource,
				checkPermissionToken("CREATE", target.WithGrantOption),
				checkPermissionToken("CONNECT", target.WithGrantOption),
				checkPermissionToken("TEMPORARY", target.WithGrantOption),
			}
			return getGrantDatabaseAllQuery, queryParams, nil
		}
		queryParams := []interface{}{target.Role, normalizeGrantPermissionToken(
			grantScope, checkPermissionToken(target.Permission, target.WithGrantOption),
		), target.Resource}
		return getGrantDatabaseQuery, queryParams, nil
	case scopes.schema:
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			queryParams := []interface{}{
				target.Role,
				target.Resource,
				checkPermissionToken("CREATE", target.WithGrantOption),
				checkPermissionToken("USAGE", target.WithGrantOption),
			}
			return getGrantSchemaAllQuery, queryParams, nil
		}
		queryParams := []interface{}{normalizeGrantPermissionToken(
			grantScope, checkPermissionToken(target.Permission, target.WithGrantOption),
		), target.Role, target.Resource}
		return getGrantSchemaQuery, queryParams, nil
	case scopes.table:
		if target.All {
			if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
				queryParams := []interface{}{
					target.Role,
					target.Schema,
					checkPermissionToken("SELECT", target.WithGrantOption),
					checkPermissionToken("INSERT", target.WithGrantOption),
					checkPermissionToken("UPDATE", target.WithGrantOption),
					checkPermissionToken("DELETE", target.WithGrantOption),
					checkPermissionToken("TRUNCATE", target.WithGrantOption),
					checkPermissionToken("REFERENCES", target.WithGrantOption),
					checkPermissionToken("TRIGGER", target.WithGrantOption),
					checkPermissionToken("MAINTAIN", target.WithGrantOption),
				}
				return getGrantAllTablesInSchemaAllQuery, queryParams, nil
			}
			queryParams := []interface{}{
				target.Role,
				target.Schema,
				checkPermissionToken(
					normalizeGrantPermissionToken(grantScope, target.Permission), target.WithGrantOption,
				),
			}
			return getGrantAllTablesInSchemaQuery, queryParams, nil
		}
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			queryParams := []interface{}{
				target.Role,
				target.Schema,
				target.Resource,
				checkPermissionToken("SELECT", target.WithGrantOption),
				checkPermissionToken("INSERT", target.WithGrantOption),
				checkPermissionToken("UPDATE", target.WithGrantOption),
				checkPermissionToken("DELETE", target.WithGrantOption),
				checkPermissionToken("TRUNCATE", target.WithGrantOption),
				checkPermissionToken("REFERENCES", target.WithGrantOption),
				checkPermissionToken("TRIGGER", target.WithGrantOption),
				checkPermissionToken("MAINTAIN", target.WithGrantOption),
			}
			return getGrantTableAllQuery, queryParams, nil
		}
		queryParams := []interface{}{normalizeGrantPermissionToken(
			grantScope, checkPermissionToken(target.Permission, target.WithGrantOption),
		), target.Role, target.Resource, target.Schema}
		return getGrantTableQuery, queryParams, nil
	case scopes.sequence:
		if target.All {
			if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
				queryParams := []interface{}{
					target.Role,
					target.Schema,
					checkPermissionToken("USAGE", target.WithGrantOption),
					checkPermissionToken("SELECT", target.WithGrantOption),
					checkPermissionToken("UPDATE", target.WithGrantOption),
				}
				return getGrantAllSequencesInSchemaAllQuery, queryParams, nil
			}
			queryParams := []interface{}{
				target.Role,
				target.Schema,
				checkPermissionToken(
					normalizeGrantPermissionToken(grantScope, target.Permission), target.WithGrantOption,
				),
			}
			return getGrantAllSequencesInSchemaQuery, queryParams, nil
		}
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			queryParams := []interface{}{
				target.Role,
				target.Schema,
				target.Resource,
				checkPermissionToken("USAGE", target.WithGrantOption),
				checkPermissionToken("SELECT", target.WithGrantOption),
				checkPermissionToken("UPDATE", target.WithGrantOption),
			}
			return getGrantSequenceAllQuery, queryParams, nil
		}
		queryParams := []interface{}{
			checkPermissionToken(normalizeGrantPermissionToken(grantScope, target.Permission), target.WithGrantOption),
			target.Role,
			target.Resource,
			target.Schema,
		}
		return getGrantSequenceQuery, queryParams, nil
	case scopes.function, scopes.procedure, scopes.routine:
		normalizedPermission := normalizeGrantPermissionToken(grantScope, target.Permission)
		if target.All {
			if normalizedPermission == "ALL" {
				queryParams := []interface{}{target.Role, target.Schema, checkPermissionToken(
					"EXECUTE", target.WithGrantOption,
				)}
				switch grantScope {
				case scopes.function:
					return getGrantAllFunctionsInSchemaAllQuery, queryParams, nil
				case scopes.procedure:
					return getGrantAllProceduresInSchemaAllQuery, queryParams, nil
				default:
					return getGrantAllRoutinesInSchemaAllQuery, queryParams, nil
				}
			}
			queryParams := []interface{}{target.Role, target.Schema, checkPermissionToken(
				normalizedPermission, target.WithGrantOption,
			)}
			switch grantScope {
			case scopes.function:
				return getGrantAllFunctionsInSchemaQuery, queryParams, nil
			case scopes.procedure:
				return getGrantAllProceduresInSchemaQuery, queryParams, nil
			default:
				return getGrantAllRoutinesInSchemaQuery, queryParams, nil
			}
		}
		queryParams := []interface{}{target.Role, target.Resource, target.Schema, checkPermissionToken(
			normalizedPermission, target.WithGrantOption,
		)}
		switch grantScope {
		case scopes.function:
			return getGrantFunctionQuery, queryParams, nil
		case scopes.procedure:
			return getGrantProcedureQuery, queryParams, nil
		default:
			return getGrantRoutineQuery, queryParams, nil
		}
	case scopes.domain:
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			return getGrantDomainAllQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
				"USAGE", target.WithGrantOption,
			)}, nil
		}
		return getGrantDomainQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
			normalizeGrantPermissionToken(
				grantScope, target.Permission,
			), target.WithGrantOption,
		)}, nil
	case scopes.fdw:
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			return getGrantFdwAllQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
				"USAGE", target.WithGrantOption,
			)}, nil
		}
		return getGrantFdwQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
			normalizeGrantPermissionToken(
				grantScope, target.Permission,
			), target.WithGrantOption,
		)}, nil
	case scopes.foreignServer:
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			return getGrantForeignServerAllQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
				"USAGE", target.WithGrantOption,
			)}, nil
		}
		return getGrantForeignServerQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
			normalizeGrantPermissionToken(
				grantScope, target.Permission,
			), target.WithGrantOption,
		)}, nil
	case scopes.language:
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			return getGrantLanguageAllQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
				"USAGE", target.WithGrantOption,
			)}, nil
		}
		return getGrantLanguageQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
			normalizeGrantPermissionToken(
				grantScope, target.Permission,
			), target.WithGrantOption,
		)}, nil
	case scopes.largeObject:
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			return getGrantLargeObjectAllQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
				"SELECT", target.WithGrantOption,
			), checkPermissionToken("UPDATE", target.WithGrantOption)}, nil
		}
		return getGrantLargeObjectQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
			normalizeGrantPermissionToken(
				grantScope, target.Permission,
			), target.WithGrantOption,
		)}, nil
	case scopes.parameter:
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			return getGrantParameterAllQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
				"SET", target.WithGrantOption,
			), checkPermissionToken("ALTER SYSTEM", target.WithGrantOption)}, nil
		}
		return getGrantParameterQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
			normalizeGrantPermissionToken(
				grantScope, target.Permission,
			), target.WithGrantOption,
		)}, nil
	case scopes.tablespace:
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			return getGrantTablespaceAllQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
				"CREATE", target.WithGrantOption,
			)}, nil
		}
		return getGrantTablespaceQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
			normalizeGrantPermissionToken(
				grantScope, target.Permission,
			), target.WithGrantOption,
		)}, nil
	case scopes.typ:
		if normalizeGrantPermissionToken(grantScope, target.Permission) == "ALL" {
			return getGrantTypeAllQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
				"USAGE", target.WithGrantOption,
			)}, nil
		}
		return getGrantTypeQuery, []interface{}{target.Role, target.Resource, checkPermissionToken(
			normalizeGrantPermissionToken(
				grantScope, target.Permission,
			), target.WithGrantOption,
		)}, nil
	default:
		return "", nil, fmt.Errorf("no query for scope: %s", target.Scope)
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
	return "", fmt.Errorf("invalid scope: %s", s)
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
			return "", nil, fmt.Errorf("invalid database permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantDatabase").Parse(setGrantDatabaseTemplate)
	case scopes.schema:
		if !slices.Contains(grantSchemaPermissions, perm) {
			return "", nil, fmt.Errorf("invalid schema permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantSchema").Parse(setGrantSchemaTemplate)
	case scopes.table:
		if !slices.Contains(grantTablePermissions, perm) {
			return "", nil, fmt.Errorf("invalid table permission: %s", target.Permission)
		}
		if target.All {
			tmpl, err = template.New("setGrantAllTables").Parse(setGrantAllTablesTemplate)
		} else {
			tmpl, err = template.New("setGrantTable").Parse(setGrantTableTemplate)
		}
	case scopes.sequence:
		if !slices.Contains(grantSequencePermissions, perm) {
			return "", nil, fmt.Errorf("invalid sequence permission: %s", target.Permission)
		}
		if target.All {
			tmpl, err = template.New("setGrantAllSequences").Parse(setGrantAllSequencesTemplate)
		} else {
			tmpl, err = template.New("setGrantSequence").Parse(setGrantSequenceTemplate)
		}
	case scopes.function, scopes.procedure, scopes.routine:
		if !slices.Contains(grantRoutinePermissions, perm) {
			return "", nil, fmt.Errorf("invalid %s permission: %s", strings.ToLower(target.Scope), target.Permission)
		}
		switch grantScope {
		case scopes.function:
			if target.All {
				tmpl, err = template.New("setGrantAllFunctions").Parse(setGrantAllFunctionsTemplate)
			} else {
				tmpl, err = template.New("setGrantFunction").Parse(setGrantFunctionTemplate)
			}
		case scopes.procedure:
			if target.All {
				tmpl, err = template.New("setGrantAllProcedures").Parse(setGrantAllProceduresTemplate)
			} else {
				tmpl, err = template.New("setGrantProcedure").Parse(setGrantProcedureTemplate)
			}
		default:
			if target.All {
				tmpl, err = template.New("setGrantAllRoutines").Parse(setGrantAllRoutinesTemplate)
			} else {
				tmpl, err = template.New("setGrantRoutine").Parse(setGrantRoutineTemplate)
			}
		}
	case scopes.domain:
		if !slices.Contains(grantDomainPermissions, perm) {
			return "", nil, fmt.Errorf("invalid domain permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantDomain").Parse(setGrantDomainTemplate)
	case scopes.fdw:
		if !slices.Contains(grantFdwPermissions, perm) {
			return "", nil, fmt.Errorf("invalid fdw permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantFdw").Parse(setGrantFdwTemplate)
	case scopes.foreignServer:
		if !slices.Contains(grantForeignServerPermissions, perm) {
			return "", nil, fmt.Errorf("invalid foreign server permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantForeignServer").Parse(setGrantForeignServerTemplate)
	case scopes.language:
		if !slices.Contains(grantLanguagePermissions, perm) {
			return "", nil, fmt.Errorf("invalid language permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantLanguage").Parse(setGrantLanguageTemplate)
	case scopes.largeObject:
		if !slices.Contains(grantLargeObjectPermissions, perm) {
			return "", nil, fmt.Errorf("invalid large object permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantLargeObject").Parse(setGrantLargeObjectTemplate)
	case scopes.parameter:
		if !slices.Contains(grantParameterPermissions, perm) {
			return "", nil, fmt.Errorf("invalid parameter permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantParameter").Parse(setGrantParameterTemplate)
	case scopes.tablespace:
		if !slices.Contains(grantTablespacePermissions, perm) {
			return "", nil, fmt.Errorf("invalid tablespace permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantTablespace").Parse(setGrantTablespaceTemplate)
	case scopes.typ:
		if !slices.Contains(grantTypePermissions, perm) {
			return "", nil, fmt.Errorf("invalid type permission: %s", target.Permission)
		}
		tmpl, err = template.New("setGrantType").Parse(setGrantTypeTemplate)
	default:
		return "", nil, fmt.Errorf("no query for scope: %s", target.Scope)
	}

	if err != nil {
		return "", nil, err
	}

	var queryBuffer bytes.Buffer
	err = tmpl.Execute(&queryBuffer, &normalizedTarget)
	if err != nil {
		return "", nil, err
	}

	return applyWithGrantOptionClause(queryBuffer.String(), normalizedTarget.WithGrantOption), nil, nil
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
			return "", nil, fmt.Errorf("invalid database permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantDatabase").Parse(setRevokeDatabaseTemplate)
	case scopes.schema:
		if !slices.Contains(grantSchemaPermissions, perm) {
			return "", nil, fmt.Errorf("invalid schema permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantSchema").Parse(setRevokeSchemaTemplate)
	case scopes.table:
		if !slices.Contains(grantTablePermissions, perm) {
			return "", nil, fmt.Errorf("invalid table permission: %s", target.Permission)
		}
		if target.All {
			tmpl, err = template.New("revokeGrantAllTables").Parse(setRevokeAllTablesTemplate)
		} else {
			tmpl, err = template.New("revokeGrantTable").Parse(setRevokeTableTemplate)
		}
	case scopes.sequence:
		if !slices.Contains(grantSequencePermissions, perm) {
			return "", nil, fmt.Errorf("invalid sequence permission: %s", target.Permission)
		}
		if target.All {
			tmpl, err = template.New("revokeGrantAllSequences").Parse(setRevokeAllSequencesTemplate)
		} else {
			tmpl, err = template.New("revokeGrantSequence").Parse(setRevokeSequenceTemplate)
		}
	case scopes.function, scopes.procedure, scopes.routine:
		if !slices.Contains(grantRoutinePermissions, perm) {
			return "", nil, fmt.Errorf("invalid %s permission: %s", strings.ToLower(target.Scope), target.Permission)
		}
		switch grantScope {
		case scopes.function:
			if target.All {
				tmpl, err = template.New("revokeGrantAllFunctions").Parse(setRevokeAllFunctionsTemplate)
			} else {
				tmpl, err = template.New("revokeGrantFunction").Parse(setRevokeFunctionTemplate)
			}
		case scopes.procedure:
			if target.All {
				tmpl, err = template.New("revokeGrantAllProcedures").Parse(setRevokeAllProceduresTemplate)
			} else {
				tmpl, err = template.New("revokeGrantProcedure").Parse(setRevokeProcedureTemplate)
			}
		default:
			if target.All {
				tmpl, err = template.New("revokeGrantAllRoutines").Parse(setRevokeAllRoutinesTemplate)
			} else {
				tmpl, err = template.New("revokeGrantRoutine").Parse(setRevokeRoutineTemplate)
			}
		}
	case scopes.domain:
		if !slices.Contains(grantDomainPermissions, perm) {
			return "", nil, fmt.Errorf("invalid domain permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantDomain").Parse(setRevokeDomainTemplate)
	case scopes.fdw:
		if !slices.Contains(grantFdwPermissions, perm) {
			return "", nil, fmt.Errorf("invalid fdw permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantFdw").Parse(setRevokeFdwTemplate)
	case scopes.foreignServer:
		if !slices.Contains(grantForeignServerPermissions, perm) {
			return "", nil, fmt.Errorf("invalid foreign server permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantForeignServer").Parse(setRevokeForeignServerTemplate)
	case scopes.language:
		if !slices.Contains(grantLanguagePermissions, perm) {
			return "", nil, fmt.Errorf("invalid language permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantLanguage").Parse(setRevokeLanguageTemplate)
	case scopes.largeObject:
		if !slices.Contains(grantLargeObjectPermissions, perm) {
			return "", nil, fmt.Errorf("invalid large object permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantLargeObject").Parse(setRevokeLargeObjectTemplate)
	case scopes.parameter:
		if !slices.Contains(grantParameterPermissions, perm) {
			return "", nil, fmt.Errorf("invalid parameter permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantParameter").Parse(setRevokeParameterTemplate)
	case scopes.tablespace:
		if !slices.Contains(grantTablespacePermissions, perm) {
			return "", nil, fmt.Errorf("invalid tablespace permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantTablespace").Parse(setRevokeTablespaceTemplate)
	case scopes.typ:
		if !slices.Contains(grantTypePermissions, perm) {
			return "", nil, fmt.Errorf("invalid type permission: %s", target.Permission)
		}
		tmpl, err = template.New("revokeGrantType").Parse(setRevokeTypeTemplate)
	default:
		return "", nil, fmt.Errorf("no revoke query for scope: %s", target.Scope)
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
