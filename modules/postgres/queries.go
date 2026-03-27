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
    has_database_privilege($1, $2, $3)
    AND has_database_privilege($1, $2, $4)
    AND has_database_privilege($1, $2, $5)
);
`
	getGrantSchemaQuery    = `SELECT has_schema_privilege($2, $3, $1);`
	getGrantSchemaAllQuery = `
SELECT (
    has_schema_privilege($1, $2, $3)
    AND has_schema_privilege($1, $2, $4)
);
`
	getGrantTableQuery    = `SELECT has_table_privilege($2, $4 || '.' || $3, $1);`
	getGrantTableAllQuery = `
SELECT (
    has_table_privilege($1, $2 || '.' || $3, $4)
    AND has_table_privilege($1, $2 || '.' || $3, $5)
    AND has_table_privilege($1, $2 || '.' || $3, $6)
    AND has_table_privilege($1, $2 || '.' || $3, $7)
    AND has_table_privilege($1, $2 || '.' || $3, $8)
    AND has_table_privilege($1, $2 || '.' || $3, $9)
    AND has_table_privilege($1, $2 || '.' || $3, $10)
    AND has_table_privilege($1, $2 || '.' || $3, $11)
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
	getGrantSequenceQuery    = `SELECT has_sequence_privilege($2, $4 || '.' || $3, $1);`
	getGrantSequenceAllQuery = `
SELECT (
    has_sequence_privilege($1, $2 || '.' || $3, $4)
    AND has_sequence_privilege($1, $2 || '.' || $3, $5)
    AND has_sequence_privilege($1, $2 || '.' || $3, $6)
);
`
	getGrantAllSequencesInSchemaQuery = `
SELECT
  COALESCE(
    bool_and(
      has_sequence_privilege(
        $1,
        format('%I.%I', sequence_schema, sequence_name),
        $3
      )
    ),
    true
  ) AS all_sequences_ok
FROM information_schema.sequences
WHERE sequence_schema = $2;
`
	getGrantAllSequencesInSchemaAllQuery = `
SELECT
  COALESCE(
    bool_and(
      has_sequence_privilege($1, format('%I.%I', sequence_schema, sequence_name), $3)
      AND has_sequence_privilege($1, format('%I.%I', sequence_schema, sequence_name), $4)
      AND has_sequence_privilege($1, format('%I.%I', sequence_schema, sequence_name), $5)
    ),
    true
  ) AS all_sequences_ok
FROM information_schema.sequences
WHERE sequence_schema = $2;
`
	getGrantFunctionQuery = `
SELECT COALESCE(
  has_function_privilege($1, to_regprocedure(format('%I.%s', $3::text, $2::text)), $4),
  false
);
`
	getGrantAllFunctionsInSchemaQuery = `
SELECT
  COALESCE(
    bool_and(
      has_function_privilege($1, p.oid, $3)
    ),
    true
  ) AS all_functions_ok
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $2
  AND p.prokind = 'f';
`
	getGrantAllFunctionsInSchemaAllQuery = `
SELECT
  COALESCE(
    bool_and(
      has_function_privilege($1, p.oid, $3)
    ),
    true
  ) AS all_functions_ok
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $2
  AND p.prokind = 'f';
`
	getGrantProcedureQuery = `
SELECT COALESCE(
  has_function_privilege($1, to_regprocedure(format('%I.%s', $3::text, $2::text)), $4),
  false
);
`
	getGrantAllProceduresInSchemaQuery = `
SELECT
  COALESCE(
    bool_and(
      has_function_privilege($1, p.oid, $3)
    ),
    true
  ) AS all_procedures_ok
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $2
  AND p.prokind = 'p';
`
	getGrantAllProceduresInSchemaAllQuery = `
SELECT
  COALESCE(
    bool_and(
      has_function_privilege($1, p.oid, $3)
    ),
    true
  ) AS all_procedures_ok
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $2
  AND p.prokind = 'p';
`
	getGrantRoutineQuery = `
SELECT COALESCE(
  has_function_privilege($1, to_regprocedure(format('%I.%s', $3::text, $2::text)), $4),
  false
);
`
	getGrantAllRoutinesInSchemaQuery = `
SELECT
  COALESCE(
    bool_and(
      has_function_privilege($1, p.oid, $3)
    ),
    true
  ) AS all_routines_ok
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $2
  AND p.prokind IN ('f', 'p');
`
	getGrantAllRoutinesInSchemaAllQuery = `
SELECT
  COALESCE(
    bool_and(
      has_function_privilege($1, p.oid, $3)
    ),
    true
  ) AS all_routines_ok
FROM pg_proc p
JOIN pg_namespace n ON n.oid = p.pronamespace
WHERE n.nspname = $2
  AND p.prokind IN ('f', 'p');
`
	getGrantDomainQuery = `SELECT has_type_privilege($1, $2, $3);`
	getGrantDomainAllQuery = `SELECT has_type_privilege($1, $2, $3);`
	getGrantFdwQuery = `SELECT has_foreign_data_wrapper_privilege($1, $2, $3);`
	getGrantFdwAllQuery = `SELECT has_foreign_data_wrapper_privilege($1, $2, $3);`
	getGrantForeignServerQuery = `SELECT has_server_privilege($1, $2, $3);`
	getGrantForeignServerAllQuery = `SELECT has_server_privilege($1, $2, $3);`
	getGrantLanguageQuery = `SELECT has_language_privilege($1, $2, $3);`
	getGrantLanguageAllQuery = `SELECT has_language_privilege($1, $2, $3);`
	getGrantLargeObjectQuery = `SELECT has_largeobject_privilege($1, $2::oid, $3);`
	getGrantLargeObjectAllQuery = `
SELECT (
  has_largeobject_privilege($1, $2::oid, $3)
  AND has_largeobject_privilege($1, $2::oid, $4)
);
`
	getGrantParameterQuery = `SELECT has_parameter_privilege($1, $2, $3);`
	getGrantParameterAllQuery = `
SELECT (
  has_parameter_privilege($1, $2, $3)
  AND has_parameter_privilege($1, $2, $4)
);
`
	getGrantTablespaceQuery = `SELECT has_tablespace_privilege($1, $2, $3);`
	getGrantTablespaceAllQuery = `SELECT has_tablespace_privilege($1, $2, $3);`
	getGrantTypeQuery = `SELECT has_type_privilege($1, $2, $3);`
	getGrantTypeAllQuery = `SELECT has_type_privilege($1, $2, $3);`
	getGrantAllTablesInSchemaAllQuery = `
SELECT
  COALESCE(
    bool_and(
      has_table_privilege($1, format('%I.%I', schemaname, tablename), $3)
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), $4)
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), $5)
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), $6)
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), $7)
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), $8)
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), $9)
      AND has_table_privilege($1, format('%I.%I', schemaname, tablename), $10)
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
	setGrantSequenceTemplate  = `GRANT {{.Permission}} ON SEQUENCE "{{.Schema}}"."{{.Resource}}" TO "{{.Role}}";`
	setGrantAllSequencesTemplate = `GRANT {{.Permission}} ON ALL SEQUENCES IN SCHEMA "{{.Schema}}" TO "{{.Role}}";`
	setGrantFunctionTemplate = `GRANT {{.Permission}} ON FUNCTION "{{.Schema}}".{{.Resource}} TO "{{.Role}}";`
	setGrantAllFunctionsTemplate = `GRANT {{.Permission}} ON ALL FUNCTIONS IN SCHEMA "{{.Schema}}" TO "{{.Role}}";`
	setGrantProcedureTemplate = `GRANT {{.Permission}} ON PROCEDURE "{{.Schema}}".{{.Resource}} TO "{{.Role}}";`
	setGrantAllProceduresTemplate = `GRANT {{.Permission}} ON ALL PROCEDURES IN SCHEMA "{{.Schema}}" TO "{{.Role}}";`
	setGrantRoutineTemplate = `GRANT {{.Permission}} ON ROUTINE "{{.Schema}}".{{.Resource}} TO "{{.Role}}";`
	setGrantAllRoutinesTemplate = `GRANT {{.Permission}} ON ALL ROUTINES IN SCHEMA "{{.Schema}}" TO "{{.Role}}";`
	setGrantDomainTemplate = `GRANT {{.Permission}} ON DOMAIN "{{.Resource}}" TO "{{.Role}}";`
	setGrantFdwTemplate = `GRANT {{.Permission}} ON FOREIGN DATA WRAPPER "{{.Resource}}" TO "{{.Role}}";`
	setGrantForeignServerTemplate = `GRANT {{.Permission}} ON FOREIGN SERVER "{{.Resource}}" TO "{{.Role}}";`
	setGrantLanguageTemplate = `GRANT {{.Permission}} ON LANGUAGE "{{.Resource}}" TO "{{.Role}}";`
	setGrantLargeObjectTemplate = `GRANT {{.Permission}} ON LARGE OBJECT {{.Resource}} TO "{{.Role}}";`
	setGrantParameterTemplate = `GRANT {{.Permission}} ON PARAMETER "{{.Resource}}" TO "{{.Role}}";`
	setGrantTablespaceTemplate = `GRANT {{.Permission}} ON TABLESPACE "{{.Resource}}" TO "{{.Role}}";`
	setGrantTypeTemplate = `GRANT {{.Permission}} ON TYPE "{{.Resource}}" TO "{{.Role}}";`
	setRevokeInstanceTemplate = `REVOKE "{{.Permission}}" FROM "{{.Role}}";`
	setRevokeDatabaseTemplate = `REVOKE {{.Permission}} ON DATABASE "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeSchemaTemplate   = `REVOKE {{.Permission}} ON SCHEMA "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeTableTemplate    = `REVOKE {{.Permission}} ON TABLE "{{.Schema}}"."{{.Resource}}" FROM "{{.Role}}";`
	setRevokeAllTablesTemplate = `REVOKE {{.Permission}} ON ALL TABLES IN SCHEMA "{{.Schema}}" FROM "{{.Role}}";`
	setRevokeSequenceTemplate = `REVOKE {{.Permission}} ON SEQUENCE "{{.Schema}}"."{{.Resource}}" FROM "{{.Role}}";`
	setRevokeAllSequencesTemplate = `REVOKE {{.Permission}} ON ALL SEQUENCES IN SCHEMA "{{.Schema}}" FROM "{{.Role}}";`
	setRevokeFunctionTemplate = `REVOKE {{.Permission}} ON FUNCTION "{{.Schema}}".{{.Resource}} FROM "{{.Role}}";`
	setRevokeAllFunctionsTemplate = `REVOKE {{.Permission}} ON ALL FUNCTIONS IN SCHEMA "{{.Schema}}" FROM "{{.Role}}";`
	setRevokeProcedureTemplate = `REVOKE {{.Permission}} ON PROCEDURE "{{.Schema}}".{{.Resource}} FROM "{{.Role}}";`
	setRevokeAllProceduresTemplate = `REVOKE {{.Permission}} ON ALL PROCEDURES IN SCHEMA "{{.Schema}}" FROM "{{.Role}}";`
	setRevokeRoutineTemplate = `REVOKE {{.Permission}} ON ROUTINE "{{.Schema}}".{{.Resource}} FROM "{{.Role}}";`
	setRevokeAllRoutinesTemplate = `REVOKE {{.Permission}} ON ALL ROUTINES IN SCHEMA "{{.Schema}}" FROM "{{.Role}}";`
	setRevokeDomainTemplate = `REVOKE {{.Permission}} ON DOMAIN "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeFdwTemplate = `REVOKE {{.Permission}} ON FOREIGN DATA WRAPPER "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeForeignServerTemplate = `REVOKE {{.Permission}} ON FOREIGN SERVER "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeLanguageTemplate = `REVOKE {{.Permission}} ON LANGUAGE "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeLargeObjectTemplate = `REVOKE {{.Permission}} ON LARGE OBJECT {{.Resource}} FROM "{{.Role}}";`
	setRevokeParameterTemplate = `REVOKE {{.Permission}} ON PARAMETER "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeTablespaceTemplate = `REVOKE {{.Permission}} ON TABLESPACE "{{.Resource}}" FROM "{{.Role}}";`
	setRevokeTypeTemplate = `REVOKE {{.Permission}} ON TYPE "{{.Resource}}" FROM "{{.Role}}";`
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
