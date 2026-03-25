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
	setRevokeInstanceTemplate = `REVOKE "{{.Permission}}" FROM "{{.Role}}";`
	setRevokeDatabaseTemplate = `REVOKE {{.Permission}} ON DATABASE "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeSchemaTemplate   = `REVOKE {{.Permission}} ON SCHEMA "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeTableTemplate    = `REVOKE {{.Permission}} ON TABLE "{{.Schema}}"."{{.Resource}}" FROM "{{.Role}}";`
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

// validatePostgresIdentifier checks that the given identifier is a valid PostgreSQL identifier.
func validatePostgresIdentifier(id string) error {
	// check non-empty
	if id == "" {
		return fmt.Errorf("identifier cannot be empty")
	}

	// check length
	if len(id) > 63 {
		return fmt.Errorf("identifier cannot be longer than 63 characters")
	}

	// Identifiers must start with a letter or underscore.
	for i, c := range id {
		if i == 0 {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
				continue
			}
			return fmt.Errorf("invalid first character in identifier: %c", c)
		}

		// Remaining characters can be letters, digits, underscore, or dollar sign.
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '_' ||
			c == '$' {
			continue
		}
		return fmt.Errorf("invalid character in identifier: %c", c)
	}
	return nil
}
