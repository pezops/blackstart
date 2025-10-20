package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	postgrestest "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

var testPg *postgrestest.PostgresContainer
var lockPg sync.Mutex

func createTestContainer(ctx context.Context) (*postgrestest.PostgresContainer, func()) {
	lockPg.Lock()
	defer lockPg.Unlock()
	if testPg != nil {
		return testPg, func() {}
	}
	_ = os.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/var/run/docker.sock")

	waitFor := func() testcontainers.CustomizeRequestOption {
		return func(req *testcontainers.GenericContainerRequest) error {
			req.WaitingFor = wait.ForListeningPort("5432/tcp").WithStartupTimeout(60 * time.Second)
			return nil
		}
	}

	var err error
	testPg, err = postgrestest.Run(
		ctx,
		"postgres:16-alpine",
		postgrestest.WithDatabase("test"),
		postgrestest.WithUsername("role"),
		postgrestest.WithPassword("password"),
		waitFor(),
	)
	teardownContainer := func() {
		err = testPg.Terminate(ctx)
		if err != nil {
			log.Printf("failed to terminate postgres container: %v", err.Error())
		}
	}

	if err != nil {
		log.Fatalf("failed to start postgres container: %v", err)
	}

	return testPg, teardownContainer
}

func createTestInstance(ctx context.Context, t *testing.T) (*sql.DB, func()) {
	if testPg == nil {
		panic("Test container must be created before creating test instance")
	}

	dsn, err := testPg.ConnectionString(ctx)
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

	return db, closeDb
}
