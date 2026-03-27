package postgres

import (
	"fmt"
	"strings"
)

const (
	getGrantInstanceQuery = `SELECT pg_has_role($2, $1, 'USAGE');`
	getGrantDatabaseQuery = `
SELECT EXISTS (
    SELECT 1
    FROM pg_database d
    WHERE d.datname = $3
      AND has_database_privilege($1, $3, $2)
);
`
	getGrantDatabaseAllQuery = `
SELECT (
    has_database_privilege($1, $2, 'CREATE')
    AND has_database_privilege($1, $2, 'CONNECT')
    AND has_database_privilege($1, $2, 'TEMPORARY')
);
`
	getGrantSchemaQuery    = `SELECT has_schema_privilege($2, $3, $1);`
	getGrantSchemaAllQuery = `
SELECT (
    has_schema_privilege($1, $2, 'CREATE')
    AND has_schema_privilege($1, $2, 'USAGE')
);
`
	getGrantTableQuery    = `SELECT has_table_privilege($2, $4 || '.' || $3, $1);`
	getGrantTableAllQuery = `
SELECT (
    has_table_privilege($1, $2 || '.' || $3, 'SELECT')
    AND has_table_privilege($1, $2 || '.' || $3, 'INSERT')
    AND has_table_privilege($1, $2 || '.' || $3, 'UPDATE')
    AND has_table_privilege($1, $2 || '.' || $3, 'DELETE')
    AND has_table_privilege($1, $2 || '.' || $3, 'TRUNCATE')
    AND has_table_privilege($1, $2 || '.' || $3, 'REFERENCES')
    AND has_table_privilege($1, $2 || '.' || $3, 'TRIGGER')
    AND has_table_privilege($1, $2 || '.' || $3, 'MAINTAIN')
);
`
	getGrantAllTablesInSchemaQuery = `
SELECT
  COALESCE(
    bool_and(
      has_table_privilege(
        $1,
        format('%I.%I', schemaname, tablename),
        $3
      )
    ),
    true
  ) AS all_tables_ok
FROM pg_tables
WHERE schemaname = $2;
`
	getGrantAllTablesInSchemaAllQuery = `
SELECT
  COALESCE(
    bool_and(
      has_table_privilege($1, format('%I.%I', schemaname, tablename), 'SELECT')
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), 'INSERT')
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), 'UPDATE')
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), 'DELETE')
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), 'TRUNCATE')
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), 'REFERENCES')
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), 'TRIGGER')
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), 'MAINTAIN')
    ),
    true
  ) AS all_tables_ok
FROM pg_tables
WHERE schemaname = $2;
`
	getRoleQuery = `
SELECT EXISTS (
	SELECT 1
	FROM pg_roles r
	WHERE r.rolname = $1
)
`
	getRoleWithOptionsQuery = `
SELECT EXISTS (
	SELECT 1
	FROM pg_roles r
	WHERE r.rolname = $1 AND r.rolinherit = $2 AND r.rolcreaterole = $3 AND r.rolcreatedb = $4 
    AND r.rolcanlogin = $5 AND r.rolreplication = $6
)
`
	setGrantInstanceTemplate  = `GRANT "{{.Permission}}" TO "{{.Role}}";`
	setGrantDatabaseTemplate  = `GRANT {{.Permission}} ON DATABASE "{{.Resource}}" TO "{{.Role}}";`
	setGrantSchemaTemplate    = `GRANT {{.Permission}} ON SCHEMA "{{.Resource}}" TO "{{.Role}}";`
	setGrantTableTemplate     = `GRANT {{.Permission}} ON TABLE "{{.Schema}}"."{{.Resource}}" TO "{{.Role}}";`
	setGrantAllTablesTemplate = `GRANT {{.Permission}} ON ALL TABLES IN SCHEMA "{{.Schema}}" TO "{{.Role}}";`
	setRevokeInstanceTemplate = `REVOKE "{{.Permission}}" FROM "{{.Role}}";`
	setRevokeDatabaseTemplate = `REVOKE {{.Permission}} ON DATABASE "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeSchemaTemplate   = `REVOKE {{.Permission}} ON SCHEMA "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeTableTemplate    = `REVOKE {{.Permission}} ON TABLE "{{.Schema}}"."{{.Resource}}" FROM "{{.Role}}";`
	setRevokeAllTablesTemplate = `REVOKE {{.Permission}} ON ALL TABLES IN SCHEMA "{{.Schema}}" FROM "{{.Role}}";`
	setRoleCreateTemplate     = `CREATE ROLE "{{.Name}}" WITH {{ if .Login }}LOGIN {{else}}NOLOGIN {{end}}{{- if .Inherit }}INHERIT {{else}}NOINHERIT {{end}}{{- if .CreateDb }}CREATEDB {{else}}NOCREATEDB {{end}}{{- if .CreateRole }}CREATEROLE {{else}}NOCREATEROLE {{end}}{{- if .Replication }}REPLICATION {{else}}NOREPLICATION {{end}};`
	setRoleUpdateTemplate     = `ALTER ROLE "{{.Name}}" WITH {{ if .Login }}LOGIN {{else}}NOLOGIN {{end}}{{- if .Inherit }}INHERIT {{else}}NOINHERIT {{end}}{{- if .CreateDb }}CREATEDB {{else}}NOCREATEDB {{end}}{{- if .CreateRole }}CREATEROLE {{else}}NOCREATEROLE {{end}}{{- if .Replication }}REPLICATION {{else}}NOREPLICATION {{end}};`
	setRoleDeleteTemplate     = `DROP ROLE "{{.Name}}";`
)

// cleanQuery converts a multi-line, user-readable SQL query into a format that is easier / cleaner
// to log from the database and application perspective.
func cleanQuery(input string) string {
	trimmed := strings.TrimSpace(input)
	cleaned := strings.ReplaceAll(trimmed, "\n", " ")
	return cleaned
}

// validatePostgresQuotedIdentifier validates identifiers that will be rendered inside double
// quotes in SQL. This permits more complex role names while rejecting characters that would break
// quoted SQL identifier rendering.
func validatePostgresQuotedIdentifier(id string) error {
	if id == "" {
		return fmt.Errorf("identifier cannot be empty")
	}
	if len(id) > 63 {
		return fmt.Errorf("identifier cannot be longer than 63 characters")
	}
	for _, c := range id {
		if c == '"' {
			return fmt.Errorf("invalid character in identifier: %c", c)
		}
		if c < 32 {
			return fmt.Errorf("invalid character in identifier: %c", c)
		}
	}
	return nil
}
