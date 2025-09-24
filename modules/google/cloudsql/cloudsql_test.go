package cloudsql

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2/google"
)

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
				creds, err := google.CredentialsFromJSON(ctx, []byte(tt.credJson))
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
