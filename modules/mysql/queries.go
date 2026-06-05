package mysql

import (
	"fmt"
	"strings"
)

const (
	getDatabaseGrantQuery = `
SELECT EXISTS (
	SELECT 1
	FROM information_schema.schema_privileges
	WHERE grantee = ?
	  AND table_schema = ?
	  AND privilege_type = ?
	  AND (? = 'NO' OR is_grantable = 'YES')
);
`

	getTableGrantQuery = `
SELECT EXISTS (
	SELECT 1
	FROM information_schema.table_privileges
	WHERE grantee = ?
	  AND table_schema = ?
	  AND table_name = ?
	  AND privilege_type = ?
	  AND (? = 'NO' OR is_grantable = 'YES')
)
OR EXISTS (
	SELECT 1
	FROM information_schema.schema_privileges
	WHERE grantee = ?
	  AND table_schema = ?
	  AND privilege_type = ?
	  AND (? = 'NO' OR is_grantable = 'YES')
);
`

	getAllTablesGrantQuery = `
SELECT NOT EXISTS (
	SELECT 1
	FROM information_schema.tables t
	WHERE t.table_schema = ?
	  AND t.table_type = 'BASE TABLE'
	  AND NOT EXISTS (
		SELECT 1
		FROM information_schema.schema_privileges p
		WHERE p.grantee = ?
		  AND p.table_schema = t.table_schema
		  AND p.privilege_type = ?
		  AND (? = 'NO' OR p.is_grantable = 'YES')
	  )
	  AND NOT EXISTS (
		SELECT 1
		FROM information_schema.table_privileges p
		WHERE p.grantee = ?
		  AND p.table_schema = t.table_schema
		  AND p.table_name = t.table_name
		  AND p.privilege_type = ?
		  AND (? = 'NO' OR p.is_grantable = 'YES')
	  )
);
`
)

// grantExistsQuery contains a parameterized grant existence query.
type grantExistsQuery struct {
	query  string
	params []any
}

// mysqlAccount identifies a MySQL account as user and host.
type mysqlAccount struct {
	User string
	Host string
}

// parseMySQLAccount parses a MySQL account string. Host defaults to % when omitted.
func parseMySQLAccount(input string) (*mysqlAccount, error) {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		return nil, fmt.Errorf("account cannot be empty")
	}

	user := trimmed
	host := "%"
	if strings.Contains(trimmed, "@") {
		parts := strings.Split(trimmed, "@")
		if len(parts) != 2 {
			return nil, fmt.Errorf("account must be formatted as user or user@host")
		}

		user = strings.TrimSpace(parts[0])
		host = strings.TrimSpace(parts[1])
	}

	if err := validateMySQLAccountPart(user, "user"); err != nil {
		return nil, err
	}

	if err := validateMySQLAccountPart(host, "host"); err != nil {
		return nil, err
	}

	return &mysqlAccount{User: user, Host: host}, nil
}

// SQL returns the account formatted for MySQL GRANT and REVOKE statements.
func (a *mysqlAccount) SQL() string {
	return fmt.Sprintf("'%s'@'%s'", escapeMySQLString(a.User), escapeMySQLString(a.Host))
}

// InformationSchema returns the account format stored by information_schema privilege tables.
func (a *mysqlAccount) InformationSchema() string {
	return a.SQL()
}

// validateMySQLAccountPart rejects account parts that cannot be safely quoted.
func validateMySQLAccountPart(value string, field string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("account %s cannot be empty", field)
	}

	for _, c := range value {
		if c < 32 {
			return fmt.Errorf("invalid character in account %s: %c", field, c)
		}
	}

	return nil
}

// escapeMySQLString escapes a value for a single-quoted MySQL string literal.
func escapeMySQLString(value string) string {
	value = strings.ReplaceAll(value, `\`, `\\`)
	return strings.ReplaceAll(value, `'`, `''`)
}

// quoteIdentifier quotes a MySQL identifier with backticks.
func quoteIdentifier(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", fmt.Errorf("identifier cannot be empty")
	}

	for _, c := range trimmed {
		if c == '`' || c < 32 {
			return "", fmt.Errorf("invalid character in identifier: %c", c)
		}
	}

	return "`" + trimmed + "`", nil
}

// renderGrantSQL returns a GRANT statement for the target.
func renderGrantSQL(target *grant) (string, error) {
	return renderGrantChangeSQL("GRANT", "TO", target)
}

// renderRevokeSQL returns a REVOKE statement for the target.
func renderRevokeSQL(target *grant) (string, error) {
	return renderGrantChangeSQL("REVOKE", "FROM", target)
}

// renderGrantChangeSQL returns a GRANT or REVOKE statement for the target.
func renderGrantChangeSQL(verb string, accountKeyword string, target *grant) (string, error) {
	account, err := parseMySQLAccount(target.Role)
	if err != nil {
		return "", err
	}

	object, err := grantObjectSQL(target)
	if err != nil {
		return "", err
	}

	sql := fmt.Sprintf("%s %s ON %s %s %s", verb, target.Permission, object, accountKeyword, account.SQL())
	if verb == "GRANT" && target.WithGrantOption {
		sql += " WITH GRANT OPTION"
	}

	return sql, nil
}

// grantObjectSQL returns the ON target for a MySQL GRANT or REVOKE statement.
func grantObjectSQL(target *grant) (string, error) {
	switch target.Scope {
	case scopes.database:
		db, err := quoteIdentifier(target.Resource)
		if err != nil {
			return "", err
		}

		return db + ".*", nil
	case scopes.table:
		db, err := quoteIdentifier(target.Schema)
		if err != nil {
			return "", err
		}

		if target.All {
			return db + ".*", nil
		}

		table, err := quoteIdentifier(target.Resource)
		if err != nil {
			return "", err
		}

		return db + "." + table, nil
	default:
		return "", fmt.Errorf("unsupported scope: %s", target.Scope)
	}
}

// getGrantExistsQueries returns parameterized check queries for the target.
func getGrantExistsQueries(target *grant) ([]*grantExistsQuery, error) {
	account, err := parseMySQLAccount(target.Role)
	if err != nil {
		return nil, err
	}

	grantable := "NO"
	if target.WithGrantOption {
		grantable = "YES"
	}

	permissions, err := grantCheckPermissions(target)
	if err != nil {
		return nil, err
	}

	queries := make([]*grantExistsQuery, 0, len(permissions))
	for _, permission := range permissions {
		existsQuery, queryErr := getGrantExistsQueryForPermission(target, account, permission, grantable)
		if queryErr != nil {
			return nil, queryErr
		}

		queries = append(queries, existsQuery)
	}

	return queries, nil
}

// grantCheckPermissions expands ALL into the concrete privileges MySQL stores.
func grantCheckPermissions(target *grant) ([]string, error) {
	if target.Permission != "ALL" {
		return []string{target.Permission}, nil
	}

	switch target.Scope {
	case scopes.database:
		return privilegesWithoutAll(grantDatabasePermissions), nil
	case scopes.table:
		return privilegesWithoutAll(grantTablePermissions), nil
	default:
		return nil, fmt.Errorf("unsupported scope: %s", target.Scope)
	}
}

// privilegesWithoutAll returns the privilege list without the ALL shorthand.
func privilegesWithoutAll(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value != "ALL" {
			out = append(out, value)
		}
	}

	return out
}

// getGrantExistsQueryForPermission returns one check query for a concrete permission.
func getGrantExistsQueryForPermission(
	target *grant, account *mysqlAccount, permission string, grantable string,
) (*grantExistsQuery, error) {
	switch target.Scope {
	case scopes.database:
		return &grantExistsQuery{
			query:  getDatabaseGrantQuery,
			params: []any{account.InformationSchema(), target.Resource, permission, grantable},
		}, nil
	case scopes.table:
		if target.All {
			return &grantExistsQuery{
				query: getAllTablesGrantQuery,
				params: []any{
					target.Schema,
					account.InformationSchema(), permission, grantable,
					account.InformationSchema(), permission, grantable,
				},
			}, nil
		}

		return &grantExistsQuery{
			query: getTableGrantQuery,
			params: []any{
				account.InformationSchema(), target.Schema, target.Resource, permission, grantable,
				account.InformationSchema(), target.Schema, permission, grantable,
			},
		}, nil
	default:
		return nil, fmt.Errorf("unsupported scope: %s", target.Scope)
	}
}
