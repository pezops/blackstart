package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"golang.org/x/oauth2/google"
	googleoauth2 "google.golang.org/api/oauth2/v2"
)

func tokenInfoEmail(ctx context.Context, creds *google.Credentials) (string, error) {
	// Use the token to query the user's identity.
	oauth2Service, err := googleoauth2.NewService(ctx)
	if err != nil {
		return "", err
	}

	token, err := creds.TokenSource.Token()
	if err != nil {
		return "", err
	}
	tokenInfoCall := oauth2Service.Tokeninfo().AccessToken(token.AccessToken)
	tokenInfo, err := tokenInfoCall.Do()
	if err != nil {
		return "", err
	}

	return tokenInfo.Email, nil
}

// DefaultCredentials returns the default Google Cloud credentials. It supports ADC (Application
// Default Credentials) and workload identity for applications running on Google Cloud. This uses
// default fallback patterns from Google libraries. The default fallback patterns, in order, are:
// 1. Environment variable GOOGLE_APPLICATION_CREDENTIALS pointing to a service account key file.
// 2. A JSON file in a well-known location created by the gcloud command-line tool.
// 3. Credentials from the metadata server on GCE, GKE, App Engine, Cloud Run, and others.
func DefaultCredentials(ctx context.Context) (*google.Credentials, error) {
	adc, err := google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	if err != nil {
		return nil, err
	}
	return adc, nil
}

// ProjectIdFromCredentials returns the Google Cloud project ID from the given credentials.
func ProjectIdFromCredentials(creds *google.Credentials) (string, error) {
	if creds.ProjectID != "" {
		return creds.ProjectID, nil
	}
	return "", errors.New("project ID not found in credentials")
}

// CurrentProject returns the current project ID. If the project is not specified explicitly
// using environment variables, it will attempt to use the default credentials to determine the
// project ID. When the credentials are used, the credentials are also returned. If the project
// is specified explicitly, the credentials returned are nil.
func CurrentProject(ctx context.Context) (string, *google.Credentials, error) {
	project := os.Getenv("GOOGLE_CLOUD_PROJECT")
	if project != "" {
		return project, nil, nil
	}

	creds, err := DefaultCredentials(ctx)
	if err != nil {
		return "", nil, err
	}

	project, err = ProjectIdFromCredentials(creds)
	if err != nil {
		return "", nil, err
	}

	return project, creds, nil
}

// AdcIamUser returns the Google Cloud IAM user associated with the current ADC (Application
// Default Credentials). This function is useful for determining the IAM user that is being used to
// make requests to Google Cloud services.
func AdcIamUser(ctx context.Context) (string, error) {
	creds, err := DefaultCredentials(ctx)
	if err != nil {
		return "", err

	}

	return IamUser(ctx, creds)
}

// IamUser returns the Google Cloud IAM user associated with the given credentials. This function
// is useful for determining the IAM user that is being used to make requests to Google Cloud
// services. The IAM user is determined by querying the OAuth2 token info endpoint.
func IamUser(ctx context.Context, creds *google.Credentials) (string, error) {
	return iamUserWithResolver(ctx, creds, tokenInfoEmail)
}

// iamUserWithResolver resolves IAM identity using credentials JSON when available, otherwise
// falling back to resolver-backed token inspection.
func iamUserWithResolver(
	ctx context.Context,
	creds *google.Credentials,
	tokenEmailResolver func(context.Context, *google.Credentials) (string, error),
) (string, error) {
	type credsJson struct {
		Type        string `json:"type,omitempty"`
		ClientEmail string `json:"client_email,omitempty"`
	}
	if len(creds.JSON) > 0 {
		var credsJsonData credsJson
		err := json.Unmarshal(creds.JSON, &credsJsonData)
		if err != nil {
			return "", fmt.Errorf("failed to parse credentials json: %w", err)
		}
		if credsJsonData.Type == "service_account" && credsJsonData.ClientEmail != "" {
			return credsJsonData.ClientEmail, nil
		}
	}

	return tokenEmailResolver(ctx, creds)
}
