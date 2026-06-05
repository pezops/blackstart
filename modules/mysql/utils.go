package mysql

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	testMySQLDatabase     = "test"
	testMySQLPassword     = "password"
	testMySQLRootPassword = "rootpassword"
	testMySQLUsername     = "role"
)

var testMySQL testcontainers.Container
var lockMySQL sync.Mutex

// createTestContainer starts or reuses the shared MySQL test container.
func createTestContainer(ctx context.Context) (testcontainers.Container, func()) {
	lockMySQL.Lock()
	defer lockMySQL.Unlock()
	if testMySQL != nil {
		return testMySQL, func() {}
	}
	_ = os.Setenv("TESTCONTAINERS_DOCKER_SOCKET_OVERRIDE", "/var/run/docker.sock")

	waitFor := wait.ForSQL(
		"3306/tcp",
		"mysql",
		func(host string, port string) string {
			port = strings.TrimSuffix(port, "/tcp")
			return fmt.Sprintf(
				"%s:%s@tcp(%s:%s)/%s?parseTime=true",
				testMySQLUsername,
				testMySQLPassword,
				host,
				port,
				testMySQLDatabase,
			)
		},
	).WithStartupTimeout(120 * time.Second).
		WithPollInterval(2 * time.Second)

	req := testcontainers.ContainerRequest{
		Image:        "mysql:8.4",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_DATABASE":      testMySQLDatabase,
			"MYSQL_PASSWORD":      testMySQLPassword,
			"MYSQL_ROOT_PASSWORD": testMySQLRootPassword,
			"MYSQL_USER":          testMySQLUsername,
		},
		WaitingFor: waitFor,
	}

	var err error
	testMySQL, err = testcontainers.GenericContainer(
		ctx,
		testcontainers.GenericContainerRequest{
			ContainerRequest: req,
			Started:          true,
		},
	)
	if err != nil {
		log.Fatalf("failed to start mysql container: %v", err)
	}

	teardownContainer := func() {
		err = testMySQL.Terminate(ctx)
		if err != nil {
			log.Printf("failed to terminate mysql container: %v", err.Error())
		}
	}

	return testMySQL, teardownContainer
}

// testMySQLConnectionParts returns the host and mapped port for the test container.
func testMySQLConnectionParts(ctx context.Context) (string, int, error) {
	host, err := testMySQL.Host(ctx)
	if err != nil {
		return "", 0, err
	}

	port, err := testMySQL.MappedPort(ctx, "3306/tcp")
	if err != nil {
		return "", 0, err
	}

	mappedPort, err := strconv.Atoi(port.Port())
	if err != nil {
		return "", 0, err
	}

	return host, mappedPort, nil
}

// testMySQLDSN returns a MySQL DSN for the shared test container.
func testMySQLDSN(ctx context.Context, username string, password string, database string) (string, error) {
	host, port, err := testMySQLConnectionParts(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true", username, password, host, port, database), nil
}
