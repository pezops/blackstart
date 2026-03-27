package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"reflect"
	"slices"
	"strings"

	"github.com/pezops/blackstart"
	"github.com/pezops/blackstart/util"
)

const defaultPrivilegesModuleID = "postgres_default_privileges"
const defaultACLObjTypeTables = "r"

type defaultPrivilegeScope string

const (
	defaultPrivilegeScopeTables       defaultPrivilegeScope = "TABLE"
	defaultPrivilegeScopeSequences    defaultPrivilegeScope = "SEQUENCE"
	defaultPrivilegeScopeFunctions    defaultPrivilegeScope = "FUNCTION"
	defaultPrivilegeScopeRoutines     defaultPrivilegeScope = "ROUTINE"
	defaultPrivilegeScopeTypes        defaultPrivilegeScope = "TYPE"
	defaultPrivilegeScopeSchemas      defaultPrivilegeScope = "SCHEMA"
	defaultPrivilegeScopeLargeObjects defaultPrivilegeScope = "LARGE_OBJECT"
)

const (
	inputForRole    = "for_role"
	inputRevokeMode = "revoke_mode"
)

var defaultPrivilegeScopes = []defaultPrivilegeScope{
	defaultPrivilegeScopeTables,
	defaultPrivilegeScopeSequences,
	defaultPrivilegeScopeFunctions,
	defaultPrivilegeScopeRoutines,
	defaultPrivilegeScopeTypes,
	defaultPrivilegeScopeSchemas,
	defaultPrivilegeScopeLargeObjects,
}

var defaultPrivilegeTablePermissions = []string{
	"SELECT", "INSERT", "UPDATE", "DELETE", "TRUNCATE", "REFERENCES", "TRIGGER", "MAINTAIN", "ALL",
}
var defaultPrivilegeSequencePermissions = []string{"USAGE", "SELECT", "UPDATE", "ALL"}
var defaultPrivilegeRoutinePermissions = []string{"EXECUTE", "ALL"}
var defaultPrivilegeTypePermissions = []string{"USAGE", "ALL"}
var defaultPrivilegeSchemaPermissions = []string{"USAGE", "CREATE", "ALL"}
var defaultPrivilegeLargeObjectPermissions = []string{"SELECT", "UPDATE", "ALL"}

type defaultPrivilegeScopeSpec struct {
	DefaultACLObjectType string
	SQLObjectName        string
	Permissions          []string
	SchemaSupported      bool
}

var defaultPrivilegeScopeSpecs = map[defaultPrivilegeScope]defaultPrivilegeScopeSpec{
	defaultPrivilegeScopeTables: {
		DefaultACLObjectType: "r",
		SQLObjectName:        "TABLES",
		Permissions:          defaultPrivilegeTablePermissions,
		SchemaSupported:      true,
	},
	defaultPrivilegeScopeSequences: {
		DefaultACLObjectType: "S",
		SQLObjectName:        "SEQUENCES",
		Permissions:          defaultPrivilegeSequencePermissions,
		SchemaSupported:      true,
	},
	defaultPrivilegeScopeFunctions: {
		DefaultACLObjectType: "f",
		SQLObjectName:        "FUNCTIONS",
		Permissions:          defaultPrivilegeRoutinePermissions,
		SchemaSupported:      true,
	},
	defaultPrivilegeScopeRoutines: {
		DefaultACLObjectType: "f",
		SQLObjectName:        "ROUTINES",
		Permissions:          defaultPrivilegeRoutinePermissions,
		SchemaSupported:      true,
	},
	defaultPrivilegeScopeTypes: {
		DefaultACLObjectType: "T",
		SQLObjectName:        "TYPES",
		Permissions:          defaultPrivilegeTypePermissions,
		SchemaSupported:      true,
	},
	defaultPrivilegeScopeSchemas: {
		DefaultACLObjectType: "n",
		SQLObjectName:        "SCHEMAS",
		Permissions:          defaultPrivilegeSchemaPermissions,
		SchemaSupported:      false,
	},
	defaultPrivilegeScopeLargeObjects: {
		DefaultACLObjectType: "L",
		SQLObjectName:        "LARGE OBJECTS",
		Permissions:          defaultPrivilegeLargeObjectPermissions,
		SchemaSupported:      false,
	},
}

type defaultPrivilegeTarget struct {
	Scope           defaultPrivilegeScope
	OwnerRole       string
	Schema          string
	Grantee         string
	Permission      string
	CheckPermissions []string
	WithGrantOption bool
	RevokeMode      string
}

type defaultPrivilegesModule struct {
	db *sql.DB
}

func init() {
	blackstart.RegisterModule(defaultPrivilegesModuleID, NewPostgresDefaultPrivileges)
}

func NewPostgresDefaultPrivileges() blackstart.Module {
	return newDefaultPrivilegesModule()
}

func newDefaultPrivilegesModule() *defaultPrivilegesModule {
	return &defaultPrivilegesModule{}
}

func (m *defaultPrivilegesModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:   defaultPrivilegesModuleID,
		Name: "PostgreSQL default privileges",
		Description: util.CleanString(
			`
Ensures PostgreSQL default privilege definitions in ['''pg_default_acl'''](https://www.postgresql.org/docs/current/catalog-pg-default-acl.html)
are present or absent. This module does not reconcile grants on existing resources. Default 
privileges apply only to new objects created under the matching '''FOR ROLE''' context.

When operation '''doesNotExist=false''', this module applies default privilege grants. When 
'''doesNotExist=true''', it removes matching default privilege entries.
`,
		),
		Requirements: []string{
			"A valid Postgres `connection` input must be provided.",
			"The database user in `connection` must have permission to execute `ALTER DEFAULT PRIVILEGES` for the configured owner role context (`FOR ROLE`).",
			"Target roles/users in `role` should exist before applying grants or revokes.",
		},
		Inputs: map[string]blackstart.InputValue{
			inputConnection: {
				Description: "Database connection.",
				Type:        reflect.TypeFor[*sql.DB](),
				Required:    true,
			},
			inputRole: {
				Description: "Role(s) receiving the default privileges.",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    true,
			},
			inputPermission: {
				Description: "Permission(s) to grant or revoke in the default-privilege definition.",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    true,
			},
			inputScope: {
				Description: "Object class for default privileges. Supported values: `TABLE`, `SEQUENCE`, `FUNCTION`, `ROUTINE`, `TYPE`, `SCHEMA`, `LARGE_OBJECT`.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputForRole: {
				Description: "Owner role(s) used in `FOR ROLE`. If omitted, current database role is used.",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    false,
			},
			inputSchema: {
				Description: "Optional schema(s) used in `IN SCHEMA`.",
				Types:       []reflect.Type{reflect.TypeFor[string](), reflect.TypeFor[[]string]()},
				Required:    false,
			},
			inputWithGrantOption: {
				Description: "Apply `WITH GRANT OPTION`. Not valid when operation `doesNotExist=true` (revoke mode).",
				Type:        reflect.TypeFor[bool](),
				Required:    false,
				Default:     false,
			},
			inputRevokeMode: {
				Description: "Revoke behavior when operation `doesNotExist=true`. Supported values: `RESTRICT`, `CASCADE`.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "RESTRICT",
			},
		},
		Outputs: map[string]blackstart.OutputValue{},
		Examples: map[string]string{
			"Grant SELECT default privilege for future tables": `id: default-privs-grant-tables
module: postgres_default_privileges
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: 
    - app_reader
    - analytics_team
  permission:
    - SELECT
    - UPDATE
  scope: TABLE
  for_role: app_owner
  schema: public
  with_grant_option: false`,
			"Revoke SELECT default privilege for future tables": `id: default-privs-revoke-tables
module: postgres_default_privileges
doesNotExist: true
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: analytics_team
  permission: UPDATE
  scope: TABLE
  for_role: app_owner
  schema: public
  revoke_mode: RESTRICT`,
			"Revoke EXECUTE default privilege from PUBLIC for future functions": `id: default-privs-revoke-functions-public
module: postgres_default_privileges
doesNotExist: true
inputs:
  connection:
    fromDependency:
      id: manage-instance
      output: connection
  role: PUBLIC
  permission: EXECUTE
  scope: FUNCTION
  for_role: admin
  revoke_mode: RESTRICT`,
		},
	}
}

func parseDefaultPrivilegeScope(value string) (defaultPrivilegeScope, error) {
	normalized := strings.ToUpper(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, " ", "_")
	switch normalized {
	case "TABLES":
		normalized = "TABLE"
	case "SEQUENCES":
		normalized = "SEQUENCE"
	case "FUNCTIONS":
		normalized = "FUNCTION"
	case "ROUTINES":
		normalized = "ROUTINE"
	case "TYPES":
		normalized = "TYPE"
	case "SCHEMAS":
		normalized = "SCHEMA"
	case "LARGE_OBJECTS":
		normalized = "LARGE_OBJECT"
	}
	scope := defaultPrivilegeScope(normalized)
	if !slices.Contains(defaultPrivilegeScopes, scope) {
		return "", fmt.Errorf("invalid %q value %q", inputScope, value)
	}
	return scope, nil
}

func normalizeRevokeMode(value string) (string, error) {
	behavior := strings.ToUpper(strings.TrimSpace(value))
	if behavior == "" {
		behavior = "RESTRICT"
	}
	if behavior != "RESTRICT" && behavior != "CASCADE" {
		return "", fmt.Errorf("invalid %q value %q", inputRevokeMode, value)
	}
	return behavior, nil
}

func validateDefaultPrivilegePermission(scope defaultPrivilegeScope, permission string) error {
	perm := normalizeDefaultPrivilegePermissionToken(permission)
	if strings.Contains(perm, ",") {
		return fmt.Errorf("invalid permission %q: comma-separated permissions are not supported", permission)
	}
	spec, ok := defaultPrivilegeScopeSpecs[scope]
	if !ok {
		return fmt.Errorf("unsupported scope %s", scope)
	}
	if !slices.Contains(spec.Permissions, perm) {
		return fmt.Errorf("invalid permission %q for scope %s", permission, scope)
	}
	return nil
}

func normalizeDefaultPrivilegePermissionToken(permission string) string {
	perm := strings.ToUpper(strings.TrimSpace(permission))
	if perm == "ALL PRIVILEGES" {
		return "ALL"
	}
	return perm
}

func expandDefaultPrivilegeCheckPermissions(scope defaultPrivilegeScope, permission string) ([]string, error) {
	spec, ok := defaultPrivilegeScopeSpecs[scope]
	if !ok {
		return nil, fmt.Errorf("unsupported scope %s", scope)
	}
	perm := normalizeDefaultPrivilegePermissionToken(permission)
	if perm != "ALL" {
		return []string{perm}, nil
	}
	out := make([]string, 0, len(spec.Permissions))
	for _, p := range spec.Permissions {
		if p == "ALL" {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

func validateDefaultPrivilegeGrantee(roleName string) error {
	if strings.EqualFold(strings.TrimSpace(roleName), "PUBLIC") {
		return nil
	}
	return validatePostgresQuotedIdentifier(roleName)
}

func validateDefaultPrivilegeOwnerRole(roleName string) error {
	return validatePostgresQuotedIdentifier(roleName)
}

func quoteRoleOrPublic(roleName string) string {
	if strings.EqualFold(strings.TrimSpace(roleName), "PUBLIC") {
		return "PUBLIC"
	}
	return fmt.Sprintf(`"%s"`, roleName)
}

func (m *defaultPrivilegesModule) setup(mctx blackstart.ModuleContext) error {
	conn, err := blackstart.ContextInputAs[*sql.DB](mctx, inputConnection, true)
	if err != nil {
		return fmt.Errorf("invalid input %s: %w", inputConnection, err)
	}
	m.db = conn
	return nil
}

func (m *defaultPrivilegesModule) resolveCurrentRole(mctx blackstart.ModuleContext) (string, error) {
	var currentRole string
	err := m.db.QueryRowContext(mctx, "SELECT current_user").Scan(&currentRole)
	if err != nil {
		return "", fmt.Errorf("failed to resolve current role: %w", err)
	}
	return currentRole, nil
}

func normalizeDefaultPrivilegesOptionalStringList(values []string) []string {
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

func (m *defaultPrivilegesModule) expandTargets(mctx blackstart.ModuleContext) ([]defaultPrivilegeTarget, error) {
	scopeRaw, err := blackstart.ContextInputAs[string](mctx, inputScope, true)
	if err != nil {
		return nil, err
	}
	scope, err := parseDefaultPrivilegeScope(scopeRaw)
	if err != nil {
		return nil, err
	}
	scopeSpec, ok := defaultPrivilegeScopeSpecs[scope]
	if !ok {
		return nil, fmt.Errorf("unsupported scope %s", scope)
	}

	withGrantOption, err := blackstart.ContextInputAs[bool](mctx, inputWithGrantOption, false)
	if err != nil {
		return nil, err
	}
	if mctx.DoesNotExist() && withGrantOption {
		return nil, fmt.Errorf("input %q is not valid when doesNotExist is true", inputWithGrantOption)
	}

	revokeModeRaw, err := blackstart.ContextInputAs[string](mctx, inputRevokeMode, false)
	if err != nil {
		return nil, err
	}
	revokeMode, err := normalizeRevokeMode(revokeModeRaw)
	if err != nil {
		return nil, err
	}
	if !mctx.DoesNotExist() && strings.TrimSpace(revokeModeRaw) != "" && !strings.EqualFold(revokeModeRaw, "RESTRICT") {
		return nil, fmt.Errorf("input %q is only used when doesNotExist is true", inputRevokeMode)
	}

	grantees, err := blackstart.ContextInputAs[[]string](mctx, inputRole, true)
	if err != nil {
		return nil, err
	}
	for i, grantee := range grantees {
		grantees[i] = strings.TrimSpace(grantee)
		if grantees[i] == "" {
			return nil, fmt.Errorf("input %q value[%d] cannot be empty", inputRole, i)
		}
		if err := validateDefaultPrivilegeGrantee(grantees[i]); err != nil {
			return nil, fmt.Errorf("input %q is invalid: %w", inputRole, err)
		}
	}

	permissions, err := blackstart.ContextInputAs[[]string](mctx, inputPermission, true)
	if err != nil {
		return nil, err
	}
	for i, permission := range permissions {
		permissions[i] = normalizeDefaultPrivilegePermissionToken(permission)
		if permissions[i] == "" {
			return nil, fmt.Errorf("input %q value[%d] cannot be empty", inputPermission, i)
		}
		if err := validateDefaultPrivilegePermission(scope, permissions[i]); err != nil {
			return nil, err
		}
	}

	ownerRoles, ownerRoleInputErr := blackstart.ContextInputAs[[]string](mctx, inputForRole, false)
	if ownerRoleInputErr != nil {
		return nil, fmt.Errorf("input %q is invalid: %w", inputForRole, ownerRoleInputErr)
	}
	ownerRoles = normalizeDefaultPrivilegesOptionalStringList(ownerRoles)
	if len(ownerRoles) == 1 && ownerRoles[0] == "" {
		currentRole, err := m.resolveCurrentRole(mctx)
		if err != nil {
			return nil, err
		}
		ownerRoles = []string{currentRole}
	}
	for _, ownerRole := range ownerRoles {
		if err := validateDefaultPrivilegeOwnerRole(ownerRole); err != nil {
			return nil, fmt.Errorf("input %q is invalid: %w", inputForRole, err)
		}
	}

	schemas, schemaInputErr := blackstart.ContextInputAs[[]string](mctx, inputSchema, false)
	if schemaInputErr != nil {
		return nil, fmt.Errorf("input %q is invalid: %w", inputSchema, schemaInputErr)
	}
	schemas = normalizeDefaultPrivilegesOptionalStringList(schemas)
	for _, schema := range schemas {
		if schema == "" {
			continue
		}
		if !scopeSpec.SchemaSupported {
			return nil, fmt.Errorf("input %q is not supported when scope is %s", inputSchema, scope)
		}
		if err := validatePostgresQuotedIdentifier(schema); err != nil {
			return nil, fmt.Errorf("input %q is invalid: %w", inputSchema, err)
		}
	}

	targets := make(
		[]defaultPrivilegeTarget,
		0,
		len(ownerRoles)*len(schemas)*len(grantees)*len(permissions),
	)
	for _, ownerRole := range ownerRoles {
		for _, schema := range schemas {
			for _, grantee := range grantees {
				for _, permission := range permissions {
					checkPermissions, expandErr := expandDefaultPrivilegeCheckPermissions(scope, permission)
					if expandErr != nil {
						return nil, expandErr
					}
					targets = append(
						targets, defaultPrivilegeTarget{
							Scope:           scope,
							OwnerRole:       ownerRole,
							Schema:          schema,
							Grantee:         grantee,
							Permission:      permission,
							CheckPermissions: checkPermissions,
							WithGrantOption: withGrantOption,
							RevokeMode:      revokeMode,
						},
					)
				}
			}
		}
	}
	return targets, nil
}

func buildAlterDefaultPrivilegesSQL(target defaultPrivilegeTarget, doesNotExist bool) string {
	scopeSpec, ok := defaultPrivilegeScopeSpecs[target.Scope]
	if !ok {
		return ""
	}
	prefix := "ALTER DEFAULT PRIVILEGES"
	prefix += fmt.Sprintf(` FOR ROLE "%s"`, target.OwnerRole)
	if target.Schema != "" {
		prefix += fmt.Sprintf(` IN SCHEMA "%s"`, target.Schema)
	}
	statementPermission := target.Permission
	if statementPermission == "ALL" {
		statementPermission = "ALL PRIVILEGES"
	}

	if !doesNotExist {
		stmt := fmt.Sprintf(
			`%s GRANT %s ON %s TO %s`,
			prefix,
			statementPermission,
			scopeSpec.SQLObjectName,
			quoteRoleOrPublic(target.Grantee),
		)
		if target.WithGrantOption {
			stmt += " WITH GRANT OPTION"
		}
		return stmt + ";"
	}
	return fmt.Sprintf(
		`%s REVOKE %s ON %s FROM %s %s;`,
		prefix,
		statementPermission,
		scopeSpec.SQLObjectName,
		quoteRoleOrPublic(target.Grantee),
		target.RevokeMode,
	)
}

// defaultPrivilegeExists reports whether a default privilege entry exists for the
// given owner/object-class/schema/grantee/privilege tuple.
func defaultPrivilegeExists(
	ctx context.Context,
	db *sql.DB,
	ownerRole string,
	objectClass string,
	schemaName string,
	grantee string,
	privilege string,
	requireGrantOption bool,
) (bool, error) {
	var granteeOID int64
	if grantee == "PUBLIC" {
		granteeOID = 0
	} else {
		err := db.QueryRowContext(
			ctx,
			`SELECT oid FROM pg_roles WHERE rolname = $1`,
			grantee,
		).Scan(&granteeOID)
		if err != nil {
			return false, fmt.Errorf("failed to resolve grantee oid: %w", err)
		}
	}

	query := `
SELECT COALESCE(
  bool_or(
    acl.grantee = $4
    AND acl.privilege_type = $5
    AND (NOT $6 OR acl.is_grantable)
  ),
  false
)
FROM pg_default_acl d
CROSS JOIN LATERAL aclexplode(d.defaclacl) acl
WHERE d.defaclrole = (SELECT oid FROM pg_roles WHERE rolname = $1)
  AND d.defaclobjtype = $2
  AND (
    ($3 = '' AND d.defaclnamespace IS NULL)
    OR d.defaclnamespace = (SELECT oid FROM pg_namespace WHERE nspname = $3)
  );
`

	var exists bool
	err := db.QueryRowContext(
		ctx,
		query,
		ownerRole,
		objectClass,
		schemaName,
		granteeOID,
		privilege,
		requireGrantOption,
	).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to query default acl state: %w", err)
	}
	return exists, nil
}

func (m *defaultPrivilegesModule) Validate(op blackstart.Operation) error {
	required := []string{inputConnection, inputRole, inputPermission, inputScope}
	for _, key := range required {
		if _, ok := op.Inputs[key]; !ok {
			return fmt.Errorf("missing required parameter: %s", key)
		}
	}

	if connInput, ok := op.Inputs[inputConnection]; ok && connInput.IsStatic() && connInput.Any() == nil {
		return fmt.Errorf("missing required parameter: %s", inputConnection)
	}

	scope := defaultPrivilegeScopeTables
	scopeSpec := defaultPrivilegeScopeSpecs[scope]
	scopeInput, hasScopeInput := op.Inputs[inputScope]
	if hasScopeInput && scopeInput.IsStatic() {
		scopeRaw, err := blackstart.InputAs[string](scopeInput, true)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputScope, err)
		}
		parsedScope, err := parseDefaultPrivilegeScope(scopeRaw)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputScope, err)
		}
		scope = parsedScope
		parsedScopeSpec, ok := defaultPrivilegeScopeSpecs[scope]
		if !ok {
			return fmt.Errorf("parameter %s is invalid: unsupported scope %s", inputScope, scope)
		}
		scopeSpec = parsedScopeSpec
	}

	roleInput := op.Inputs[inputRole]
	if roleInput.IsStatic() {
		roles, err := blackstart.InputAs[[]string](roleInput, true)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputRole, err)
		}
		for _, role := range roles {
			if err := validateDefaultPrivilegeGrantee(strings.TrimSpace(role)); err != nil {
				return fmt.Errorf("parameter %s is invalid: %w", inputRole, err)
			}
		}
	}

	permissionInput := op.Inputs[inputPermission]
	if permissionInput.IsStatic() {
		permissions, err := blackstart.InputAs[[]string](permissionInput, true)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputPermission, err)
		}
		for _, permission := range permissions {
			if err := validateDefaultPrivilegePermission(scope, normalizeDefaultPrivilegePermissionToken(permission)); err != nil {
				return fmt.Errorf("parameter %s is invalid: %w", inputPermission, err)
			}
		}
	}

	if withGrantOptionInput, ok := op.Inputs[inputWithGrantOption]; ok && withGrantOptionInput.IsStatic() {
		withGrantOption, err := blackstart.InputAs[bool](withGrantOptionInput, false)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputWithGrantOption, err)
		}
		if op.DoesNotExist && withGrantOption {
			return fmt.Errorf("parameter %s is invalid: not valid when doesNotExist is true", inputWithGrantOption)
		}
	}

	if forRoleInput, ok := op.Inputs[inputForRole]; ok && forRoleInput.IsStatic() {
		owners, err := blackstart.InputAs[[]string](forRoleInput, false)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputForRole, err)
		}
		for _, owner := range owners {
			trimmed := strings.TrimSpace(owner)
			if trimmed == "" {
				continue
			}
			if err := validateDefaultPrivilegeOwnerRole(trimmed); err != nil {
				return fmt.Errorf("parameter %s is invalid: %w", inputForRole, err)
			}
		}
	}

	if schemaInput, ok := op.Inputs[inputSchema]; ok && schemaInput.IsStatic() {
		schemas, err := blackstart.InputAs[[]string](schemaInput, false)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputSchema, err)
		}
		for _, schema := range schemas {
			trimmed := strings.TrimSpace(schema)
			if trimmed == "" {
				continue
			}
			if !scopeSpec.SchemaSupported {
				return fmt.Errorf("parameter %s is invalid: not supported when scope is %s", inputSchema, scope)
			}
			if err := validatePostgresQuotedIdentifier(trimmed); err != nil {
				return fmt.Errorf("parameter %s is invalid: %w", inputSchema, err)
			}
		}
	}

	revokeModeInput, hasRevokeModeInput := op.Inputs[inputRevokeMode]
	if hasRevokeModeInput && revokeModeInput.IsStatic() {
		revokeModeRaw, err := blackstart.InputAs[string](revokeModeInput, false)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputRevokeMode, err)
		}
		revokeMode, err := normalizeRevokeMode(revokeModeRaw)
		if err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", inputRevokeMode, err)
		}
		if !op.DoesNotExist && strings.TrimSpace(revokeModeRaw) != "" && revokeMode != "RESTRICT" {
			return fmt.Errorf("parameter %s is invalid: only used when doesNotExist is true", inputRevokeMode)
		}
	}

	return nil
}

func (m *defaultPrivilegesModule) Check(mctx blackstart.ModuleContext) (bool, error) {
	if err := m.setup(mctx); err != nil {
		return false, err
	}
	targets, err := m.expandTargets(mctx)
	if err != nil {
		return false, err
	}

	for _, target := range targets {
		desired := !mctx.DoesNotExist()
		scopeSpec := defaultPrivilegeScopeSpecs[target.Scope]
		for _, checkPermission := range target.CheckPermissions {
			exists, err := defaultPrivilegeExists(
				mctx,
				m.db,
				target.OwnerRole,
				scopeSpec.DefaultACLObjectType,
				target.Schema,
				target.Grantee,
				checkPermission,
				target.WithGrantOption,
			)
			if err != nil {
				return false, err
			}
			if exists != desired {
				return false, nil
			}
		}
	}
	return true, nil
}

func (m *defaultPrivilegesModule) Set(mctx blackstart.ModuleContext) error {
	if err := m.setup(mctx); err != nil {
		return err
	}
	targets, err := m.expandTargets(mctx)
	if err != nil {
		return err
	}

	for _, target := range targets {
		query := buildAlterDefaultPrivilegesSQL(target, mctx.DoesNotExist())
		if _, err := m.db.ExecContext(mctx, query); err != nil {
			return fmt.Errorf("failed to apply default privilege statement %q: %w", query, err)
		}
	}
	return nil
}
