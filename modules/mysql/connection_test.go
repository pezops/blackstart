package mysql

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

// capturingModuleContext records module outputs while preserving normal context behavior.
type capturingModuleContext struct {
	blackstart.ModuleContext
	outputs map[string]interface{}
}

// Output records the output value and delegates to the wrapped ModuleContext.
func (c *capturingModuleContext) Output(key string, value interface{}) error {
	if c.outputs == nil {
		c.outputs = map[string]interface{}{}
	}
	c.outputs[key] = value
	return c.ModuleContext.Output(key, value)
}

// TestConnectionValidate verifies static MySQL connection validation.
func TestConnectionValidate(t *testing.T) {
	mod := connectionModule{}

	op := blackstart.Operation{Inputs: map[string]blackstart.Input{}}
	err := mod.Validate(op)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required parameter: username")

	op = blackstart.Operation{Inputs: map[string]blackstart.Input{
		inputUsername: blackstart.NewInputFromValue(""),
	}}
	err = mod.Validate(op)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parameter username is invalid")

	op = blackstart.Operation{Inputs: map[string]blackstart.Input{
		inputUsername: blackstart.NewInputFromValue("user"),
	}}
	err = mod.Validate(op)
	require.NoError(t, err)
}

// TestConnectionCheckCreatesTarget verifies defaulted connection target creation.
func TestConnectionCheckCreatesTarget(t *testing.T) {
	mod := connectionModule{}
	op := &blackstart.Operation{
		Id:     "test",
		Module: "mysql_connection",
		Name:   "Test MySQL connection",
		Inputs: map[string]blackstart.Input{
			inputHost:     blackstart.NewInputFromValue("localhost"),
			inputUsername: blackstart.NewInputFromValue("user"),
		},
	}
	mctx := blackstart.OpContext(context.Background(), op)

	check, err := mod.Check(mctx)
	require.NoError(t, err)
	require.False(t, check)

	assert.Equal(t, "localhost", mod.target.host)
	assert.Equal(t, 3306, mod.target.port)
	assert.Equal(t, "mysql", mod.target.database)
	assert.Equal(t, "user", mod.target.username)
	assert.Equal(t, "", mod.target.password)
	assert.Equal(t, "false", mod.target.tls)
}

// TestConnectionSetError verifies connection failures are surfaced.
func TestConnectionSetError(t *testing.T) {
	mod := connectionModule{}
	op := &blackstart.Operation{
		Id:     "test",
		Module: "mysql_connection",
		Inputs: map[string]blackstart.Input{
			inputHost:     blackstart.NewInputFromValue("localhost"),
			inputPort:     blackstart.NewInputFromValue(1),
			inputDatabase: blackstart.NewInputFromValue("db"),
			inputUsername: blackstart.NewInputFromValue("user"),
			inputPassword: blackstart.NewInputFromValue("pass"),
			inputTLS:      blackstart.NewInputFromValue("false"),
		},
	}

	require.NoError(t, mod.Validate(*op))
	ctx := blackstart.OpContext(context.Background(), op)

	check, err := mod.Check(ctx)
	require.NoError(t, err)
	require.False(t, check)

	err = mod.Set(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "error pinging database")
}

// TestConnectionSet verifies the module opens a usable MySQL connection.
func TestConnectionSet(t *testing.T) {
	mctx := context.Background()
	mod, ctx := testConnectionModuleContext(t, mctx)

	check, err := mod.Check(ctx)
	require.NoError(t, err)
	require.False(t, check)

	err = mod.Set(ctx)
	require.NoError(t, err)
	require.NotNil(t, mod.db)
	require.NoError(t, mod.db.Ping())
}

// TestConnectionSet_EmitsConnectionOutput verifies the connection output is emitted.
func TestConnectionSet_EmitsConnectionOutput(t *testing.T) {
	mctx := context.Background()
	mod, baseCtx := testConnectionModuleContext(t, mctx)
	ctx := &capturingModuleContext{ModuleContext: baseCtx}

	check, err := mod.Check(ctx)
	require.NoError(t, err)
	require.False(t, check)

	err = mod.Set(ctx)
	require.NoError(t, err)
	require.NotNil(t, mod.db)

	value, ok := ctx.outputs[outputConnection]
	require.True(t, ok)
	outDb, ok := value.(*sql.DB)
	require.True(t, ok)
	require.Equal(t, mod.db, outDb)
}

// TestConnectionClose_ClosesConnection verifies Close releases the active database handle.
func TestConnectionClose_ClosesConnection(t *testing.T) {
	mctx := context.Background()
	mod, ctx := testConnectionModuleContext(t, mctx)

	check, err := mod.Check(ctx)
	require.NoError(t, err)
	require.False(t, check)
	require.NoError(t, mod.Set(ctx))
	require.NotNil(t, mod.db)

	require.NoError(t, mod.Close())
	require.Nil(t, mod.db)
}

// testConnectionModuleContext returns a connection module wired to the test container.
func testConnectionModuleContext(t *testing.T, ctx context.Context) (*connectionModule, blackstart.ModuleContext) {
	host, port, err := testMySQLConnectionParts(ctx)
	require.NoError(t, err)

	mod := connectionModule{}
	op := &blackstart.Operation{
		Id:     "test",
		Module: "mysql_connection",
		Name:   "Test MySQL connection",
		Inputs: map[string]blackstart.Input{
			inputHost:     blackstart.NewInputFromValue(host),
			inputPort:     blackstart.NewInputFromValue(port),
			inputDatabase: blackstart.NewInputFromValue(testMySQLDatabase),
			inputUsername: blackstart.NewInputFromValue(testMySQLUsername),
			inputPassword: blackstart.NewInputFromValue(testMySQLPassword),
			inputTLS:      blackstart.NewInputFromValue("false"),
		},
	}
	require.NoError(t, mod.Validate(*op))

	return &mod, blackstart.OpContext(ctx, op)
}
