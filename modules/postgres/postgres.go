package postgres

import "github.com/pezops/blackstart"

const (
	inputHost            = "host"
	inputPort            = "port"
	inputDatabase        = "database"
	inputUsername        = "username"
	inputPassword        = "password"
	inputSslMode         = "sslmode"
	inputConnection      = "connection"
	inputRole            = "role"
	inputPermission      = "permission"
	inputSchema          = "schema"
	inputResource        = "resource"
	inputScope           = "scope"
	inputAll             = "all"
	inputWithGrantOption = "with_grant_option"
	inputName            = "name"
	inputCreateDb        = "create_db"
	inputCreateRole      = "create_role"
	inputInherit         = "inherit"
	inputLogin           = "login"
	inputReplication     = "replication"

	outputConnection = "connection"
)

func init() {
	blackstart.RegisterPathName("postgres", "PostgreSQL")
}
