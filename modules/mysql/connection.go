package mysql

import (
	"database/sql"
	"fmt"
	"io"
	"reflect"

	gomysql "github.com/go-sql-driver/mysql"

	"github.com/pezops/blackstart"
)

var requiredConnectionParameters = []string{inputUsername}
var _ blackstart.Module = &connectionModule{}
var _ io.Closer = &connectionModule{}

func init() {
	blackstart.RegisterModule("mysql_connection", NewMySQLConnection)
}

// NewMySQLConnection creates a new instance of the MySQL connection module.
func NewMySQLConnection() blackstart.Module {
	return &connectionModule{}
}

// connection represents a connection to a MySQL database.
type connection struct {
	// host is the hostname or IP address of the MySQL server.
	host string
	// port is the port number of the MySQL server.
	port int
	// database is the name of the MySQL database to connect to.
	database string
	// username is the username to connect to the MySQL database.
	username string
	// password is the password to connect to the MySQL database.
	password string
	// tls is the TLS mode to use when connecting to the MySQL database.
	tls string
}

// connectionModule creates and emits a MySQL database connection.
type connectionModule struct {
	db     *sql.DB
	target *connection
}

// Info returns metadata describing the MySQL connection module.
func (c *connectionModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:          "mysql_connection",
		Name:        "MySQL connection",
		Description: "Connection to a MySQL database.",
		Requirements: []string{
			"The MySQL server must be reachable from the Blackstart runtime.",
			"The provided credentials must be valid for the target database.",
			"The provided user must have permission to connect to the target database.",
			"TLS settings must match the server configuration.",
		},
		Inputs: map[string]blackstart.InputValue{
			inputHost: {
				Description: "Hostname or IP address of the MySQL server.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "localhost",
			},
			inputPort: {
				Description: "Port number of the MySQL server.",
				Type:        reflect.TypeFor[int](),
				Required:    false,
				Default:     3306,
			},
			inputDatabase: {
				Description: "Name of the MySQL database to connect to.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "mysql",
			},
			inputUsername: {
				Description: "Username to connect to the MySQL database.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputPassword: {
				Description: "Password to connect to the MySQL database.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
			inputTLS: {
				Description: "TLS mode to use when connecting to the MySQL database. Examples: `false`, `true`, `skip-verify`, or a registered TLS config name.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "false",
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputConnection: {
				Description: "The connection details to the MySQL database.",
				Type:        reflect.TypeFor[*sql.DB](),
			},
		},
		Examples: map[string]string{
			"Connect to a database": `id: connect-db
module: mysql_connection
inputs:
  host: db.example.com
  database: app
  username: admin`,
		},
	}
}

// Validate checks whether an operation contains valid MySQL connection inputs.
func (c *connectionModule) Validate(op blackstart.Operation) error {
	for _, p := range requiredConnectionParameters {
		o, ok := op.Inputs[p]
		if !ok {
			return fmt.Errorf("missing required parameter: %s", p)
		}
		if !o.IsStatic() {
			continue
		}
		if _, err := blackstart.InputAs[string](o, true); err != nil {
			return fmt.Errorf("parameter %s is invalid: %w", p, err)
		}
	}
	return nil
}

// Check creates the target connection configuration and always returns false.
func (c *connectionModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	if err := c.createTargetConnection(ctx); err != nil {
		return false, err
	}
	return false, nil
}

// Set opens the MySQL connection, verifies it, and emits it as an output.
func (c *connectionModule) Set(ctx blackstart.ModuleContext) error {
	cfg := gomysql.NewConfig()
	cfg.User = c.target.username
	cfg.Passwd = c.target.password
	cfg.Net = "tcp"
	cfg.Addr = fmt.Sprintf("%s:%d", c.target.host, c.target.port)
	cfg.DBName = c.target.database
	cfg.TLSConfig = c.target.tls
	cfg.ParseTime = true

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	if err = db.PingContext(ctx); err != nil {
		_ = db.Close()
		return fmt.Errorf("error pinging database: %w", err)
	}
	c.db = db
	if err = ctx.Output(outputConnection, c.db); err != nil {
		_ = c.db.Close()
		c.db = nil
		return err
	}
	return nil
}

// Close releases the active database connection held by the module.
func (c *connectionModule) Close() error {
	if c.db == nil {
		return nil
	}
	err := c.db.Close()
	c.db = nil
	return err
}

// createTargetConnection creates the target connection from the module context inputs.
func (c *connectionModule) createTargetConnection(ctx blackstart.ModuleContext) error {
	host, err := blackstart.ContextInputAs[string](ctx, inputHost, true)
	if err != nil {
		return err
	}

	port, err := blackstart.ContextInputAs[int64](ctx, inputPort, true)
	if err != nil {
		return err
	}

	database, err := blackstart.ContextInputAs[string](ctx, inputDatabase, true)
	if err != nil {
		return err
	}

	username, err := blackstart.ContextInputAs[string](ctx, inputUsername, true)
	if err != nil {
		return err
	}

	password, err := blackstart.ContextInputAs[string](ctx, inputPassword, false)
	if err != nil {
		return err
	}

	tls, err := blackstart.ContextInputAs[string](ctx, inputTLS, true)
	if err != nil {
		return err
	}

	c.target = &connection{
		host:     host,
		port:     int(port),
		database: database,
		username: username,
		password: password,
		tls:      tls,
	}

	return nil
}
