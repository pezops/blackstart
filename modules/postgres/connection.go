package postgres

import (
	"database/sql"
	"errors"
	"fmt"
	"reflect"

	"github.com/lib/pq"

	"github.com/pezops/blackstart"
)

var requiredConnectionParameters = []string{inputUsername}

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
		Inputs: map[string]blackstart.InputValue{
			inputHost: {
				Description: "Hostname or IP address of the PostgreSQL server.",
				Type:        reflect.TypeOf(""),
				Required:    false,
				Default:     "localhost",
			},
			inputPort: {
				Description: "port number of the PostgreSQL server.",
				Type:        reflect.TypeOf(0),
				Required:    false,
				Default:     5432,
			},
			inputDatabase: {
				Description: "Name of the PostgreSQL database to connect to.",
				Type:        reflect.TypeOf(""),
				Required:    false,
				Default:     "postgres",
			},
			inputUsername: {
				Description: "username to connect to the PostgreSQL database.",
				Type:        reflect.TypeOf(""),
				Required:    true,
			},
			inputPassword: {
				Description: "password to connect to the PostgreSQL database.",
				Type:        reflect.TypeOf(""),
				Required:    false,
			},
			inputSslMode: {
				Description: "SSL mode to use when connecting to the PostgreSQL database. Options are 'disable', 'prefer', 'require', 'verify-ca', 'verify-full'.",
				Type:        reflect.TypeOf(""),
				Required:    false,
				Default:     "prefer",
			},
		},
		Outputs: map[string]blackstart.OutputValue{
			outputConnection: {
				Description: "The connection details to the PostgreSQL database.",
				Type:        reflect.TypeOf(&sql.DB{}),
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
			if o.String() == "" {
				return fmt.Errorf("parameter %s cannot be empty", p)
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
	return nil
}

// createTargetConnection creates the target connection from the module context inputs.
func (c *connectionModule) createTargetConnection(ctx blackstart.ModuleContext) error {
	host, err := ctx.Input(inputHost)
	if err != nil {
		return err
	}

	port, err := ctx.Input(inputPort)
	if err != nil {
		return err
	}

	database, err := ctx.Input(inputDatabase)
	if err != nil {
		return err
	}

	username, err := ctx.Input(inputUsername)
	if err != nil {
		return err
	}

	password, err := ctx.Input(inputPassword)
	if err != nil {
		return err
	}

	sslmode, err := ctx.Input(inputSslMode)
	if err != nil {
		return err
	}

	c.target = &connection{
		host:     host.String(),
		port:     int(port.Number()),
		database: database.String(),
		username: username.String(),
		password: password.String(),
		sslMode:  sslmode.String(),
	}

	return nil
}
