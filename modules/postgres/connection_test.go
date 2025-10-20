package postgres

import (
	"context"
	"net/url"
	"strconv"
	"strings"
	"testing"

	_ "github.com/lib/pq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pezops/blackstart"
)

func TestConnectionValidate(t *testing.T) {
	mod := connectionModule{}
	// Missing username
	op := blackstart.Operation{Inputs: map[string]blackstart.Input{}}
	err := mod.Validate(op)
	require.Error(t, err)
	require.Contains(t, err.Error(), "missing required parameter: username")

	// Empty username
	op = blackstart.Operation{Inputs: map[string]blackstart.Input{
		inputUsername: blackstart.NewInputFromValue(""),
	}}
	err = mod.Validate(op)
	require.Error(t, err)
	require.Contains(t, err.Error(), "parameter username cannot be empty")

	// Valid username
	op = blackstart.Operation{Inputs: map[string]blackstart.Input{
		inputUsername: blackstart.NewInputFromValue("user"),
	}}
	err = mod.Validate(op)
	require.NoError(t, err)
}

func TestConnectionCheckCreatesTarget(t *testing.T) {
	mod := connectionModule{}
	op := &blackstart.Operation{
		Id:     "test",
		Module: "postgres_connection",
		Name:   "Test Postgres connection",
		Inputs: map[string]blackstart.Input{
			inputHost:     blackstart.NewInputFromValue("localhost"),
			inputUsername: blackstart.NewInputFromValue("user"),
		}}

	ictx := blackstart.InputsToContext(context.Background(), op.Inputs)

	mctx := blackstart.OpContext(ictx, op)

	check, err := mod.Check(mctx)
	require.NoError(t, err)
	require.False(t, check)

	assert.Equal(t, "localhost", mod.target.host)
	assert.Equal(t, 5432, mod.target.port)
	assert.Equal(t, "postgres", mod.target.database)
	assert.Equal(t, "user", mod.target.username)
	assert.Equal(t, "", mod.target.password)
}

func TestConnectionSetError(t *testing.T) {
	mod := connectionModule{}
	op := blackstart.Operation{Inputs: map[string]blackstart.Input{
		inputHost:     blackstart.NewInputFromValue("localhost"),
		inputPort:     blackstart.NewInputFromValue(1),
		inputDatabase: blackstart.NewInputFromValue("db"),
		inputUsername: blackstart.NewInputFromValue("user"),
		inputPassword: blackstart.NewInputFromValue("pass"),
		inputSslMode:  blackstart.NewInputFromValue("disable"),
	}}

	err := mod.Validate(op)
	require.NoError(t, err)

	ctx := blackstart.InputsToContext(context.Background(), op.Inputs)

	// Check should succeed in creating target
	check, err := mod.Check(ctx)
	require.NoError(t, err)
	require.False(t, check)

	// Set should fail to connect
	err = mod.Set(ctx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "connection refused")
}

func TestConnectionSet(t *testing.T) {
	pctx := context.Background()

	dsn, err := testPg.ConnectionString(pctx)
	require.NoError(t, err)

	parse, err := url.Parse(dsn)
	require.NoError(t, err)

	pgHost := strings.Split(parse.Host, ":")[0]
	pgPort, err := strconv.Atoi(parse.Port())
	require.NoError(t, err)
	pgUsername := parse.User.Username()
	pgPassword, _ := parse.User.Password()
	pgDatabase := strings.TrimPrefix(parse.Path, "/")

	mod := connectionModule{}
	op := &blackstart.Operation{
		Id:     "test",
		Module: "postgres_connection",
		Name:   "Test Postgres connection",
		Inputs: map[string]blackstart.Input{
			inputHost:     blackstart.NewInputFromValue(pgHost),
			inputPort:     blackstart.NewInputFromValue(pgPort),
			inputDatabase: blackstart.NewInputFromValue(pgDatabase),
			inputUsername: blackstart.NewInputFromValue(pgUsername),
			inputPassword: blackstart.NewInputFromValue(pgPassword),
			inputSslMode:  blackstart.NewInputFromValue("disable"),
		}}

	err = mod.Validate(*op)
	require.NoError(t, err)

	ctx := blackstart.InputsToContext(pctx, op.Inputs)

	// Check should succeed in creating target
	check, err := mod.Check(ctx)
	require.NoError(t, err)
	require.False(t, check)

	// Set should succeed in connecting
	err = mod.Set(ctx)
	require.NoError(t, err)
	require.NotNil(t, mod.db)

	// Verify the connection is usable
	err = mod.db.Ping()
	require.NoError(t, err)
}
