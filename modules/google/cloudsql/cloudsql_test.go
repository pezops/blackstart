package cloudsql

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2/google"
)

func testCredentialsTypeFromJSON(t *testing.T, credJSON string) google.CredentialsType {
	t.Helper()
	var payload struct {
		Type string `json:"type"`
	}
	err := json.Unmarshal([]byte(credJSON), &payload)
	if err != nil {
		t.Fatalf("failed to parse test credentials JSON: %v", err)
	}
	if payload.Type == "" {
		t.Fatalf("test credentials JSON missing type field")
	}
	return google.CredentialsType(payload.Type)
}

// TestPostgresIamUser tests the postgresIamUser function with different types of credentials.
func TestPostgresIamUser(t *testing.T) {

	tests := []struct {
		name     string
		credJson string
		want     string
		fails    bool
	}{
		{
			// Cannot get the user from these ADC. Use the oauth2 service and userinfo API.
			name: "gcloud_auth_adc",
			credJson: `{
  "account" : "",
  "client_id" : "32555940559.apps.googleusercontent.com",
  "client_secret" : "...",
  "refresh_token" : "...",
  "type" : "authorized_user",
  "universe_domain" : "googleapis.com"
}`,
			want:  "foo",
			fails: true,
		},
		{
			name: "service_account",
			credJson: `{
  "type": "service_account",
  "project_id": "test-proj",
  "private_key_id": "4630db5271ea7c0724e72cb74a05d26e18e38b0f",
  "private_key": "-----BEGIN PRIVATE KEY-----\nMIIEvgIBADANBgkqhkiG9\n-----END PRIVATE KEY-----\n",
  "client_email": "test-svc-account-cloud@test-proj.iam.gserviceaccount.com",
  "client_id": "874927474950604929841",
  "auth_uri": "https://accounts.google.com/o/oauth2/auth",
  "token_uri": "https://oauth2.googleapis.com/token",
  "auth_provider_x509_cert_url": "https://www.googleapis.com/oauth2/v1/certs",
  "client_x509_cert_url": "https://www.googleapis.com/robot/v1/metadata/x509/test-svc-account-cloud%40test-proj.iam.gserviceaccount.com",
  "universe_domain": "googleapis.com"
}`,
			want:  "test-svc-account-cloud@test-proj.iam",
			fails: false,
		},
	}

	for _, tt := range tests {
		t.Run(
			tt.name, func(t *testing.T) {
				ctx := context.Background()
				creds, err := google.CredentialsFromJSONWithTypeAndParams(
					ctx,
					[]byte(tt.credJson),
					testCredentialsTypeFromJSON(t, tt.credJson),
					google.CredentialsParams{},
				)
				if err != nil {
					t.Errorf("Failed to create credentials: %v", err)
					return
				}
				got, err := postgresIamUser(ctx, creds)
				if tt.fails {
					assert.Equal(t, "", got)
					assert.Error(t, err)
					return
				}

				assert.Equal(t, tt.want, got)
				assert.NoError(t, err)
			},
		)
	}
}

func TestCloudsqlPostgresIamDsn(t *testing.T) {
	instanceIdentifier := "project:region:instance"
	dbname := "testdb"
	username := "testuser"
	expected := "host=project:region:instance user=testuser dbname=testdb sslmode=disable"
	actual := cloudsqlPostgresIamDsn(instanceIdentifier, dbname, username)
	assert.Equal(t, expected, actual)
}

func TestCloudsqlPostgresBuiltInDsn(t *testing.T) {
	instanceIdentifier := "project:region:instance"
	dbname := "testdb"
	username := "testuser"
	password := "testpass"
	expected := "host=project:region:instance user=testuser password=testpass dbname=testdb sslmode=disable"
	actual := cloudsqlPostgresBuiltInDsn(instanceIdentifier, dbname, username, password)
	assert.Equal(t, expected, actual)
}

func TestTargetConnectionConfig_connectionIdentifier(t *testing.T) {
	t.Run(
		"should return identifier if already set", func(t *testing.T) {
			cfg := &connectionConfig{
				identifier: "project:region:instance",
			}
			identifier, err := cfg.connectionIdentifier(context.Background())
			require.NoError(t, err)
			assert.Equal(t, "project:region:instance", identifier)
		},
	)
}

func TestIamUserType(t *testing.T) {
	t.Run(
		"service account from cloudsql iam username suffix", func(t *testing.T) {
			got := iamUserType(nil, "blackstart-sa@test-cr-249905.iam")
			assert.Equal(t, userCloudIamServiceAccount, got)
		},
	)

	t.Run(
		"service account from full gsa email suffix", func(t *testing.T) {
			got := iamUserType(nil, "blackstart-sa@test-cr-249905.iam.gserviceaccount.com")
			assert.Equal(t, userCloudIamServiceAccount, got)
		},
	)

	t.Run(
		"authorized user from email identity", func(t *testing.T) {
			got := iamUserType(nil, "person@example.com")
			assert.Equal(t, userCloudIamUser, got)
		},
	)

	t.Run(
		"non-email .iam suffix does not classify as service account", func(t *testing.T) {
			got := iamUserType(nil, "bob.iam")
			assert.Equal(t, userCloudIamUser, got)
		},
	)

	t.Run(
		"fallback to credentials json type service account", func(t *testing.T) {
			creds := &google.Credentials{
				JSON: []byte(`{"type":"service_account","client_email":"svc@test-proj.iam.gserviceaccount.com"}`),
			}
			got := iamUserType(creds, "")
			assert.Equal(t, userCloudIamServiceAccount, got)
		},
	)

	t.Run(
		"fallback to credentials json type authorized user", func(t *testing.T) {
			creds := &google.Credentials{
				JSON: []byte(`{"type":"authorized_user"}`),
			}
			got := iamUserType(creds, "")
			assert.Equal(t, userCloudIamUser, got)
		},
	)
}

func TestNormalizeCloudSQLServiceAccountUsername(t *testing.T) {
	t.Run(
		"accepts existing cloudsql format", func(t *testing.T) {
			got, err := normalizeCloudSQLServiceAccountUsername("blackstart-sa@test-cr-249905.iam", "test-cr-249905")
			require.NoError(t, err)
			assert.Equal(t, "blackstart-sa@test-cr-249905.iam", got)
		},
	)

	t.Run(
		"converts full gsa email", func(t *testing.T) {
			got, err := normalizeCloudSQLServiceAccountUsername(
				"blackstart-sa@test-cr-249905.iam.gserviceaccount.com",
				"test-cr-249905",
			)
			require.NoError(t, err)
			assert.Equal(t, "blackstart-sa@test-cr-249905.iam", got)
		},
	)

	t.Run(
		"expands bare username using project", func(t *testing.T) {
			got, err := normalizeCloudSQLServiceAccountUsername("blackstart-sa", "test-cr-249905")
			require.NoError(t, err)
			assert.Equal(t, "blackstart-sa@test-cr-249905.iam", got)
		},
	)

	t.Run(
		"rejects non service-account email format", func(t *testing.T) {
			_, err := normalizeCloudSQLServiceAccountUsername("person@example.com", "test-cr-249905")
			require.Error(t, err)
		},
	)
}
