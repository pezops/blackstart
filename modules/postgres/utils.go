package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	postgrestest "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func createTestContainer(ctx context.Context, t *testing.T) (*postgrestest.PostgresContainer, func()) {
	_ = os.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/var/run/docker.sock")

	waitFor := func() testcontainers.CustomizeRequestOption {
		return func(req *testcontainers.GenericContainerRequest) error {
			req.WaitingFor = wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second)
			return nil
		}
	}

	postgresContainer, err := postgrestest.Run(
		ctx,
		"postgres:16-alpine",
		postgrestest.WithDatabase("test"),
		postgrestest.WithUsername("role"),
		postgrestest.WithPassword("password"),
		waitFor(),
	)
	teardownContainer := func() {
		err = postgresContainer.Terminate(ctx)
		if err != nil {
			t.Logf("failed to terminate postgres container: %v", err.Error())
		}
	}

	if err != nil {
		t.Fatalf("failed to start postgres container: %v", err)
	}

	return postgresContainer, teardownContainer

}

func createTestInstance(ctx context.Context, t *testing.T) (*sql.DB, func()) {
	postgresContainer, teardownContainer := createTestContainer(ctx, t)

	dsn, err := postgresContainer.ConnectionString(ctx)
	if err != nil {
		t.Fatalf("failed to get connection string: %v", err)
	}

	dsnUrl, err := url.Parse(dsn)
	if err != nil {
		t.Fatalf("failed to parse connection string: %v", err)
	}
	query := dsnUrl.Query()
	query.Set("sslmode", "disable")
	dsnUrl.RawQuery = query.Encode()

	dsn = dsnUrl.String()

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		panic(fmt.Sprintf("Failed to open a DB connection: %v", err))
	}
	closeDb := func() {
		err = db.Close()
		if err != nil {
			t.Logf("failed to close database connection: %v", err.Error())
		}
	}

	teardown := func() {
		closeDb()
		teardownContainer()
	}
	return db, teardown
}
