package cloudsql

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/pezops/blackstart"
	"github.com/stretchr/testify/require"
	"google.golang.org/api/option"
	"google.golang.org/api/sqladmin/v1"
)

type testError struct {
	message string
}

func (e *testError) Error() string {
	return e.message
}

// fakeCloudSQLAdmin implements the Cloud SQL Admin REST operations used by the modules.
type fakeCloudSQLAdmin struct {
	t        *testing.T
	server   *httptest.Server
	instance *sqladmin.DatabaseInstance
	users    []*sqladmin.User
	inserted []*sqladmin.User
	requests []string
	fail     map[string]int
	mu       sync.Mutex
}

// newFakeCloudSQLAdmin starts a stateful fake Cloud SQL Admin API server.
func newFakeCloudSQLAdmin(t *testing.T, databaseVersion string) *fakeCloudSQLAdmin {
	t.Helper()
	f := &fakeCloudSQLAdmin{
		t: t,
		instance: &sqladmin.DatabaseInstance{
			Name:            "instance",
			Project:         "project",
			Region:          "us-central1",
			DatabaseVersion: databaseVersion,
			Settings: &sqladmin.Settings{DatabaseFlags: []*sqladmin.DatabaseFlags{
				{Name: iamFlagForVersion(databaseVersion), Value: "on"},
			}},
		},
		fail: map[string]int{},
	}
	f.server = httptest.NewServer(http.HandlerFunc(f.serveHTTP))
	t.Cleanup(f.server.Close)
	return f
}

// iamFlagForVersion returns the IAM database authentication flag for a Cloud SQL version.
func iamFlagForVersion(databaseVersion string) string {
	if instanceEngine(databaseVersion) == "MYSQL" {
		return "cloudsql_iam_authentication"
	}
	return "cloudsql.iam_authentication"
}

// runtime returns a Cloud SQL runtime connected to the fake Admin API and supplied database opener.
func (f *fakeCloudSQLAdmin) runtime(opener func(string, string) (*sql.DB, error)) *cloudSQLRuntime {
	if opener == nil {
		opener = func(driver, dsn string) (*sql.DB, error) {
			return nil, fmt.Errorf("unexpected database open: driver=%s dsn=%s", driver, dsn)
		}
	}
	return &cloudSQLRuntime{
		newSQLAdminService: func(ctx context.Context) (*sqladmin.Service, error) {
			return sqladmin.NewService(
				ctx,
				option.WithEndpoint(f.server.URL+"/"),
				option.WithoutAuthentication(),
			)
		},
		openDB: opener,
	}
}

// serveHTTP handles the Cloud SQL Admin API operations used by the unit tests.
func (f *fakeCloudSQLAdmin) serveHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()

	key := r.Method + " " + r.URL.Path
	f.requests = append(f.requests, key)
	if status := f.fail[key]; status != 0 {
		http.Error(w, http.StatusText(status), status)
		return
	}

	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/instances/instance"):
		writeJSON(f.t, w, f.instance)
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/instances/instance/users"):
		writeJSON(f.t, w, &sqladmin.UsersListResponse{Items: cloneUsers(f.users)})
	case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/instances/instance/users"):
		var user sqladmin.User
		require.NoError(f.t, json.NewDecoder(r.Body).Decode(&user))
		requestUser := user
		f.inserted = append(f.inserted, &requestUser)
		if instanceEngine(f.instance.DatabaseVersion) == "MYSQL" &&
			(user.Type == userCloudIamUser || user.Type == userCloudIamServiceAccount) {
			user.IamEmail = user.Name
			user.Name, _ = mysqlIamUser(user.Name)
		}
		f.users = append(f.users, &user)
		writeJSON(f.t, w, &sqladmin.Operation{})
	case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/instances/instance/users"):
		f.deleteUser(r.URL.Query())
		writeJSON(f.t, w, &sqladmin.Operation{})
	default:
		f.t.Errorf("unexpected Cloud SQL Admin API request: %s", key)
		http.Error(w, "unexpected request: "+key, http.StatusNotFound)
	}
}

// deleteUser removes a matching user from the fake Cloud SQL instance.
func (f *fakeCloudSQLAdmin) deleteUser(query url.Values) {
	name := query.Get("name")
	host := query.Get("host")
	filtered := f.users[:0]
	for _, user := range f.users {
		if (user.Name == name || user.IamEmail == name) && (host == "" || user.Host == host) {
			continue
		}
		filtered = append(filtered, user)
	}
	f.users = filtered
}

// requestCount returns the number of matching requests received by the fake Admin API.
func (f *fakeCloudSQLAdmin) requestCount(method, pathSuffix string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	count := 0
	for _, request := range f.requests {
		if strings.HasPrefix(request, method+" ") && strings.HasSuffix(request, pathSuffix) {
			count++
		}
	}
	return count
}

// cloneUsers returns independent copies of the supplied Cloud SQL users.
func cloneUsers(users []*sqladmin.User) []*sqladmin.User {
	cloned := make([]*sqladmin.User, 0, len(users))
	for _, user := range users {
		copy := *user
		cloned = append(cloned, &copy)
	}
	return cloned
}

// writeJSON writes a JSON response and fails the test if encoding fails.
func writeJSON(t *testing.T, w http.ResponseWriter, value any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	require.NoError(t, json.NewEncoder(w).Encode(value))
}

// expectedDBOpen describes one expected database open call and its sqlmock database.
type expectedDBOpen struct {
	driver      string
	dsn         string
	dsnContains []string
	db          *sql.DB
	mock        sqlmock.Sqlmock
}

// queuedDBOpener returns mocked databases in the order they are expected to be opened.
type queuedDBOpener struct {
	t     *testing.T
	opens []*expectedDBOpen
	mu    sync.Mutex
}

// newQueuedDBOpener creates an empty ordered database opener.
func newQueuedDBOpener(t *testing.T) *queuedDBOpener {
	t.Helper()
	return &queuedDBOpener{t: t}
}

// expect adds an exact driver and DSN expectation to the database opener queue.
func (o *queuedDBOpener) expect(driver, dsn string) (*sql.DB, sqlmock.Sqlmock) {
	o.t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(o.t, err)
	o.opens = append(o.opens, &expectedDBOpen{driver: driver, dsn: dsn, db: db, mock: mock})
	return db, mock
}

// expectContaining adds a driver expectation with required DSN substrings.
func (o *queuedDBOpener) expectContaining(driver string, dsnContains ...string) (*sql.DB, sqlmock.Sqlmock) {
	o.t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(o.t, err)
	o.opens = append(
		o.opens, &expectedDBOpen{
			driver:      driver,
			dsnContains: dsnContains,
			db:          db,
			mock:        mock,
		},
	)
	return db, mock
}

// open validates and consumes the next expected database open call.
func (o *queuedDBOpener) open(driver, dsn string) (*sql.DB, error) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.opens) == 0 {
		return nil, fmt.Errorf("unexpected database open: driver=%s dsn=%s", driver, dsn)
	}
	expected := o.opens[0]
	o.opens = o.opens[1:]
	if expected.driver != driver || expected.dsn != "" && expected.dsn != dsn {
		return nil, fmt.Errorf(
			"unexpected database open: got driver=%s dsn=%s, want driver=%s dsn=%s",
			driver, dsn, expected.driver, expected.dsn,
		)
	}
	for _, substring := range expected.dsnContains {
		if !strings.Contains(dsn, substring) {
			return nil, fmt.Errorf("database DSN %q does not contain expected value %q", dsn, substring)
		}
	}
	return expected.db, nil
}

// verify fails the test when expected database open calls remain.
func (o *queuedDBOpener) verify() {
	o.t.Helper()
	require.Empty(o.t, o.opens, "not all expected database connections were opened")
}

// testCloudSQLUserOperation creates a standard Cloud SQL user test operation.
func testCloudSQLUserOperation(userName, userType string) blackstart.Operation {
	return blackstart.Operation{
		Inputs: map[string]blackstart.Input{
			inputInstance: blackstart.NewInputFromValue("instance"),
			inputProject:  blackstart.NewInputFromValue("project"),
			inputUser:     blackstart.NewInputFromValue(userName),
			inputUserType: blackstart.NewInputFromValue(userType),
		},
		Module: "google_cloudsql_user",
	}
}

// testManagedInstanceOperation creates a standard managed-instance test operation.
func testManagedInstanceOperation(userName string) blackstart.Operation {
	return blackstart.Operation{
		Inputs: map[string]blackstart.Input{
			inputInstance:       blackstart.NewInputFromValue("instance"),
			inputProject:        blackstart.NewInputFromValue("project"),
			inputUser:           blackstart.NewInputFromValue(userName),
			inputConnectionType: blackstart.NewInputFromValue("PUBLIC_IP"),
		},
		Module: "google_cloudsql_managed_instance",
	}
}
