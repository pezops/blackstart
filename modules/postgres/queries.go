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
	getGrantSchemaQuery = `SELECT has_schema_privilege($2, $3, $1);`
	getGrantTableQuery  = `SELECT has_table_privilege($2, $4 || '.' || $3, $1);`
	getRoleQuery        = `
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
	setGrantTableTemplate     = `GRANT {{.Permission}} ON TABLE "{{.Resource}}" TO "{{.Role}}";`
	setRevokeInstanceTemplate = `REVOKE "{{.Permission}}" FROM "{{.Role}}";`
	setRevokeDatabaseTemplate = `REVOKE {{.Permission}} ON DATABASE "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeSchemaTemplate   = `REVOKE {{.Permission}} ON SCHEMA "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeTableTemplate    = `REVOKE {{.Permission}} ON TABLE "{{.Resource}}" FROM "{{.Role}}";`
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

	// check allowed characters
	for _, c := range id {
		if (c >= 'a' && c <= 'z') ||
			(c >= '0' && c <= '9') ||
			c == '-' ||
			c == '$' {
			// valid character
			continue
		} else {
			return fmt.Errorf("invalid character in identifier: %c", c)
		}

	}
	return nil
}
