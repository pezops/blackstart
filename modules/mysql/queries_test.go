package mysql

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestParseMySQLAccount verifies account parsing and default host behavior.
func TestParseMySQLAccount(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantUser string
		wantHost string
		wantErr  string
	}{
		{name: "user_only", input: "app", wantUser: "app", wantHost: "%"},
		{name: "user_and_host", input: "app@localhost", wantUser: "app", wantHost: "localhost"},
		{name: "trims_parts", input: " app @ % ", wantUser: "app", wantHost: "%"},
		{name: "empty", input: "", wantErr: "account cannot be empty"},
		{name: "too_many_at_symbols", input: "a@b@c", wantErr: "account must be formatted"},
		{name: "empty_host", input: "app@", wantErr: "account host cannot be empty"},
		{name: "empty_user", input: "@localhost", wantErr: "account user cannot be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			account, err := parseMySQLAccount(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantUser, account.User)
			require.Equal(t, tt.wantHost, account.Host)
		})
	}
}

// TestQuoteIdentifier verifies MySQL identifier quoting.
func TestQuoteIdentifier(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{name: "valid", input: "orders", want: "`orders`"},
		{name: "trims", input: " orders ", want: "`orders`"},
		{name: "empty", input: "", wantErr: "identifier cannot be empty"},
		{name: "backtick", input: "bad`name", wantErr: "invalid character"},
		{name: "control", input: "bad\nname", wantErr: "invalid character"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := quoteIdentifier(tt.input)
			if tt.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestRenderGrantSQL verifies MySQL GRANT statement rendering.
func TestRenderGrantSQL(t *testing.T) {
	tests := []struct {
		name   string
		target *grant
		want   string
	}{
		{
			name: "database",
			target: &grant{
				Role:       "app",
				Permission: "SELECT",
				Resource:   "appdb",
				Scope:      scopes.database,
			},
			want: "GRANT SELECT ON `appdb`.* TO 'app'@'%'",
		},
		{
			name: "table",
			target: &grant{
				Role:       "app@localhost",
				Permission: "UPDATE",
				Schema:     "appdb",
				Resource:   "orders",
				Scope:      scopes.table,
			},
			want: "GRANT UPDATE ON `appdb`.`orders` TO 'app'@'localhost'",
		},
		{
			name: "all_tables_with_grant_option",
			target: &grant{
				Role:            "app",
				Permission:      "SELECT",
				Schema:          "appdb",
				Scope:           scopes.table,
				All:             true,
				WithGrantOption: true,
			},
			want: "GRANT SELECT ON `appdb`.* TO 'app'@'%' WITH GRANT OPTION",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := renderGrantSQL(tt.target)
			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}

// TestRenderRevokeSQL verifies MySQL REVOKE statement rendering.
func TestRenderRevokeSQL(t *testing.T) {
	got, err := renderRevokeSQL(&grant{
		Role:       "app",
		Permission: "SELECT",
		Schema:     "appdb",
		Resource:   "orders",
		Scope:      scopes.table,
	})
	require.NoError(t, err)
	require.Equal(t, "REVOKE SELECT ON `appdb`.`orders` FROM 'app'@'%'", got)
}

// TestGetGrantExistsQueries verifies parameterized grant checks.
func TestGetGrantExistsQueries(t *testing.T) {
	t.Run("single_table_permission", func(t *testing.T) {
		queries, err := getGrantExistsQueries(&grant{
			Role:       "app",
			Permission: "SELECT",
			Schema:     "appdb",
			Resource:   "orders",
			Scope:      scopes.table,
		})
		require.NoError(t, err)
		require.Len(t, queries, 1)
		require.Equal(t, []any{"'app'@'%'", "appdb", "orders", "SELECT", "NO", "'app'@'%'", "appdb", "SELECT", "NO"}, queries[0].params)
	})

	t.Run("all_expands_concrete_permissions", func(t *testing.T) {
		queries, err := getGrantExistsQueries(&grant{
			Role:       "app",
			Permission: "ALL",
			Resource:   "appdb",
			Scope:      scopes.database,
		})
		require.NoError(t, err)
		require.Len(t, queries, len(grantDatabasePermissions)-1)
		require.Equal(t, []any{"'app'@'%'", "appdb", "SELECT", "NO"}, queries[0].params)
	})
}
