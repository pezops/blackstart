package mysql

import "github.com/pezops/blackstart"

const (
	inputHost            = "host"
	inputPort            = "port"
	inputDatabase        = "database"
	inputUsername        = "username"
	inputPassword        = "password"
	inputTLS             = "tls"
	inputConnection      = "connection"
	inputRole            = "role"
	inputPermission      = "permission"
	inputSchema          = "schema"
	inputResource        = "resource"
	inputScope           = "scope"
	inputAll             = "all"
	inputWithGrantOption = "with_grant_option"

	outputConnection = "connection"
)

func init() {
	blackstart.RegisterPathName("mysql", "MySQL")
}
