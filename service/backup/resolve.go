package backup

import (
	"context"
	"fmt"
	"net/http"
	"os"
)

// CredentialResolver loads a credential by reference (e.g. "keychain:obk/r2-access-key")
// and falls back to the given environment variable.
type CredentialResolver func(ref, envVar string) (string, error)

// GoogleClientFactory creates an authenticated HTTP client for Google APIs.
type GoogleClientFactory func(ctx context.Context, cfg GoogleClientConfig) (*http.Client, error)

type GoogleClientConfig struct {
	CredentialsFile string
	TokenDBPath     string
	Scopes          []string
}

// ResolveBackendOpts holds the dependencies needed to resolve a backup backend.
type ResolveBackendOpts struct {
	ResolveCred    CredentialResolver
	GoogleClient   GoogleClientFactory
	BackupDest     string
	R2Bucket       string
	R2Endpoint     string
	R2AccessRef    string
	R2SecretRef    string
	GDriveFolderID string
}

// ResolveBackend creates the appropriate backend from config.
func ResolveBackend(ctx context.Context, opts ResolveBackendOpts) (Backend, error) {
	switch opts.BackupDest {
	case "r2":
		return resolveR2(ctx, opts)
	case "gdrive":
		return resolveGDrive(ctx, opts)
	default:
		return nil, fmt.Errorf("unknown backup destination: %q", opts.BackupDest)
	}
}

func resolveR2(_ context.Context, opts ResolveBackendOpts) (Backend, error) {
	if opts.R2Bucket == "" || opts.R2Endpoint == "" {
		return nil, fmt.Errorf("R2 config missing")
	}

	accessKey, err := opts.ResolveCred(opts.R2AccessRef, "OBK_R2_ACCESS_KEY")
	if err != nil {
		return nil, fmt.Errorf("resolve R2 access key: %w", err)
	}
	secretKey, err := opts.ResolveCred(opts.R2SecretRef, "OBK_R2_SECRET_KEY")
	if err != nil {
		return nil, fmt.Errorf("resolve R2 secret key: %w", err)
	}

	endpoint := opts.R2Endpoint
	if e := os.Getenv("OBK_R2_ENDPOINT"); e != "" {
		endpoint = e
	}
	bucket := opts.R2Bucket
	if b := os.Getenv("OBK_R2_BUCKET"); b != "" {
		bucket = b
	}

	return NewR2Backend(endpoint, accessKey, secretKey, bucket)
}

func resolveGDrive(ctx context.Context, opts ResolveBackendOpts) (Backend, error) {
	if opts.GDriveFolderID == "" {
		return nil, fmt.Errorf("Google Drive folder ID not configured")
	}
	if opts.GoogleClient == nil {
		return nil, fmt.Errorf("Google client factory not provided")
	}

	httpClient, err := opts.GoogleClient(ctx, GoogleClientConfig{Scopes: []string{"https://www.googleapis.com/auth/drive.file"}})
	if err != nil {
		return nil, fmt.Errorf("get Drive client: %w", err)
	}

	return NewGDriveBackend(ctx, httpClient, opts.GDriveFolderID)
}
