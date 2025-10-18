package cloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleoauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

var currentUserInfo *googleoauth2.Userinfo

// CurrentUserInfo returns the current user's information. This function caches the user info
// after the first call so that it doesn't need to be retrieved again. Additionally, the
// expectation is that the user info will not change during the lifetime of the application.
func CurrentUserInfo(ctx context.Context) (*googleoauth2.Userinfo, error) {
	if currentUserInfo != nil {
		return currentUserInfo, nil
	}

	// Retrieve the Application Default Credentials (ADC)
	creds, err := google.FindDefaultCredentials(ctx, googleoauth2.UserinfoEmailScope)
	if err != nil {
		return nil, fmt.Errorf("failed to find default credentials: %w", err)
	}

	// Create an OAuth2 service using the credentials
	oauth2Service, err := googleoauth2.NewService(ctx, option.WithCredentials(creds))
	if err != nil {
		return nil, fmt.Errorf("failed to create OAuth2 service: %w", err)
	}

	// Call the Userinfo API to get the principal's email
	currentUserInfo, err = oauth2Service.Userinfo.V2.Me.Get().Do()
	if err != nil {
		return nil, fmt.Errorf("failed to get user info: %w", err)
	}

	return currentUserInfo, nil
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

// HttpClient returns an HTTP client that is authenticated to Google Cloud with the given
// credentials from Google Cloud.
func HttpClient(creds *google.Credentials) (*http.Client, error) {
	ts := creds.TokenSource
	if ts == nil {
		return nil, errors.New("no token provided to client")
	}
	return &http.Client{
		Transport: &oauth2.Transport{
			Base:   http.DefaultTransport,
			Source: ts,
		},
	}, nil
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
	type credsJson struct {
		Type        string `json:"type,omitempty"`
		ClientEmail string `json:"client_email,omitempty"`
	}
	var credsJsonData credsJson
	err := json.Unmarshal(creds.JSON, &credsJsonData)
	if err != nil {
		return "", err
	}

	if credsJsonData.Type == "service_account" {
		return credsJsonData.ClientEmail, nil
	}

	// Use the token to query the user's identity
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
