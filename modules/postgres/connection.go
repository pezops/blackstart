package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/lib/pq"

	"github.com/pezops/blackstart"
)

var requiredConnectionParameters = []string{inputUsername}
var _ blackstart.Module = &connectionModule{}
var _ io.Closer = &connectionModule{}

func init() {
	blackstart.RegisterModule("postgres_connection", NewPostgresConnection)
}

func NewPostgresConnection() blackstart.Module {
	return &connectionModule{}
}

// connection represents a connection to a PostgreSQL database
type connection struct {
	// host is the hostname or IP address of the PostgreSQL server
	host string
	// port is the port number of the PostgreSQL server
	port int
	// database is the Name of the PostgreSQL database to connect to
	database string
	// username is the username to connect to the PostgreSQL database
	username string
	// password is the password to connect to the PostgreSQL database
	password string
	// sslMode is the SSL mode to use when connecting to the PostgreSQL database
	sslMode string
}

type connectionModule struct {
	//op     *blackstart.Operation
	db     *sql.DB
	target *connection
}

func (c *connectionModule) Info() blackstart.ModuleInfo {
	return blackstart.ModuleInfo{
		Id:          "postgres_connection",
		Name:        "PostgreSQL connection",
		Description: "Connection to a PostgreSQL database.",
		Requirements: []string{
			"The PostgreSQL server must be reachable from the Blackstart runtime.",
			"The provided credentials must be valid for the target database.",
			"The provided user must have permission to connect to the target database.",
			"TLS and `sslmode` settings must match the server configuration.",
		},
		Inputs: map[string]blackstart.InputValue{
			inputHost: {
				Description: "Hostname or IP address of the PostgreSQL server.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "localhost",
			},
			inputPort: {
				Description: "port number of the PostgreSQL server.",
				Type:        reflect.TypeFor[int](),
				Required:    false,
				Default:     5432,
			},
			inputDatabase: {
				Description: "Name of the PostgreSQL database to connect to.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "postgres",
			},
			inputUsername: {
				Description: "username to connect to the PostgreSQL database.",
				Type:        reflect.TypeFor[string](),
				Required:    true,
			},
			inputPassword: {
				Description: "password to connect to the PostgreSQL database.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
			},
			inputSslMode: {
				Description: "SSL mode to use when connecting to the PostgreSQL database. Options are 'disable', 'prefer', 'require', 'verify-ca', 'verify-full'.",
				Type:        reflect.TypeFor[string](),
				Required:    false,
				Default:     "prefer",
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputConnection: {
				Description: "The connection details to the PostgreSQL database.",
				Type:        reflect.TypeFor[*sql.DB](),
			},
		},
		Examples: map[string]string{
			"Connect to a database": `id: connect-db
module: postgres_connection
inputs:
  host: db.example.com
  database: mydb
  username: admin`,
		},
	}
}

func (c *connectionModule) Validate(op blackstart.Operation) error {

	for _, p := range requiredConnectionParameters {
		if o, ok := op.Inputs[p]; !ok {
			return fmt.Errorf("missing required parameter: %s", p)
		} else {
			if !o.IsStatic() {
				continue
			}
			_, err := blackstart.InputAs[string](o, true)
			if err != nil {
				return fmt.Errorf("parameter %s is invalid: %w", p, err)
			}
		}
	}

	return nil
}

func (c *connectionModule) Check(ctx blackstart.ModuleContext) (bool, error) {
	err := c.createTargetConnection(ctx)
	if err != nil {
		return false, err
	}

	return false, nil
}

func (c *connectionModule) Set(ctx blackstart.ModuleContext) error {
	// Connect to the postgres database and set c.db to the connection
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.target.host, c.target.port, c.target.username, c.target.password, c.target.database, c.target.sslMode,
	)
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return fmt.Errorf("error connecting to database: %w", err)
	}
	err = db.PingContext(ctx)
	if err != nil {
		var pqErr *pq.Error
		if errors.As(err, &pqErr) {
			return fmt.Errorf("error pinging database: %s", pqErr.Message)
		}
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

	sslmode, err := blackstart.ContextInputAs[string](ctx, inputSslMode, true)
	if err != nil {
		return err
	}

	c.target = &connection{
		host:     host,
		port:     int(port),
		database: database,
		username: username,
		password: password,
		sslMode:  sslmode,
	}

	return nil
}
