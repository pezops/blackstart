package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCleanQuery(t *testing.T) {
	tests := []struct {
		name string
		in   string
		out  string
	}{
		{
			name: "basic_query",
			in: `
SELECT * FROM table
WHERE column = $1
`,
			out: "SELECT * FROM table WHERE column = $1",
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				result := cleanQuery(tt.in)
				if result != tt.out {
					assert.Equal(t, tt.out, result)
				}
			},
		)
	}

}

func TestValidateIdentifier(t *testing.T) {
	tests := []struct {
		name       string
		identifier string
		valid      bool
	}{
		{
			name:       "valid_identifier",
			identifier: "public",
			valid:      true,
		},
		{
			name:       "invalid_identifier",
			identifier: "public;",
			valid:      false,
		},
	}
	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				err := validatePostgresIdentifier(tt.identifier)
				if tt.valid {
					require.NoError(t, err)
				} else {
					require.Error(t, err)
				}
			},
		)
	}
}
