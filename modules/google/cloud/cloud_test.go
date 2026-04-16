package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/googleapi"
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

func TestProjectIdFromCredentials(t *testing.T) {

	tests := []struct {
		name     string
		credJson string
		want     string
		fails    bool
	}{
		{
			// Cannot get project ID from ADC using `glcoud auth application-default login`
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
			want:  "test-proj",
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
				got, err := ProjectIdFromCredentials(creds)
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

func TestIamUser(t *testing.T) {

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
			want:  "test-svc-account-cloud@test-proj.iam.gserviceaccount.com",
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
				got, err := IamUser(ctx, creds)
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

func TestIamUser_ServiceAccountFromJSON(t *testing.T) {
	creds := &google.Credentials{
		JSON: []byte(`{"type":"service_account","client_email":"svc@test-proj.iam.gserviceaccount.com"}`),
	}

	got, err := iamUserWithResolver(
		context.Background(),
		creds,
		func(_ context.Context, _ *google.Credentials) (string, error) {
			return "", errors.New("should not be called")
		},
	)
	require.NoError(t, err)
	require.Equal(t, "svc@test-proj.iam.gserviceaccount.com", got)
}

func TestIamUser_EmptyJSONFallsBackToTokenInspection(t *testing.T) {
	creds := &google.Credentials{}

	got, err := iamUserWithResolver(
		context.Background(),
		creds,
		func(_ context.Context, _ *google.Credentials) (string, error) {
			return "wi-principal@example.com", nil
		},
	)
	require.NoError(t, err)
	require.Equal(t, "wi-principal@example.com", got)
}

func TestSanitizeGoogleAPIError_UsesErrorDescriptionFromBody(t *testing.T) {
	err := sanitizeGoogleAPIError(
		"token info",
		&googleapi.Error{
			Code:    400,
			Message: "Bad Request",
			Body:    `{"error_description":"Invalid Value"}`,
		},
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Invalid Value")
	require.NotContains(t, err.Error(), "access_token=")
	require.NotContains(t, err.Error(), "https://")
}

func TestSanitizeGoogleAPIError_GenericFallbackDoesNotExposeWrappedError(t *testing.T) {
	err := sanitizeGoogleAPIError(
		"id token",
		errors.New(`Post "https://www.googleapis.com/oauth2/v2/tokeninfo?access_token=secret-token&foo=bar": context deadline exceeded`),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown error while calling id token endpoint")
	require.NotContains(t, err.Error(), "secret-token")
	require.NotContains(t, err.Error(), "https://www.googleapis.com/oauth2/v2/tokeninfo")

	err = sanitizeGoogleAPIError(
		"id token",
		errors.New(`Get "https://oauth2.googleapis.com/tokeninfo?id_token=secret-id-token": unauthorized`),
	)
	require.Error(t, err)
	require.Contains(t, err.Error(), "unknown error while calling id token endpoint")
	require.NotContains(t, err.Error(), "secret-id-token")
	require.NotContains(t, err.Error(), "https://oauth2.googleapis.com/tokeninfo")
}

func TestExtractGoogleAPIErrorDescription_OptionalFields(t *testing.T) {
	require.Equal(t, "", extractGoogleAPIErrorDescription(`{}`))
	require.Equal(t, "", extractGoogleAPIErrorDescription(`{"foo":"bar"}`))
	require.Equal(
		t,
		"nested-invalid",
		extractGoogleAPIErrorDescription(`{"error":{"error_description":"nested-invalid"}}`),
	)
}
