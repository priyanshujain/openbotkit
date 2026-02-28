package gmail

import (
	"context"
	"fmt"
	"net/http"

	gapi "google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

func newGmailService(ctx context.Context, httpClient *http.Client) (*gapi.Service, error) {
	srv, err := gapi.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, fmt.Errorf("create gmail service: %w", err)
	}
	return srv, nil
}
