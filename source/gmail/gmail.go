package gmail

import (
	"context"
	"fmt"
	"log"

	"github.com/priyanshujain/openbotkit/source"
	"github.com/priyanshujain/openbotkit/store"
)

// Gmail is a data source that connects to Gmail accounts.
type Gmail struct {
	cfg Config
}

// New creates a new Gmail source with the given configuration.
func New(cfg Config) *Gmail {
	return &Gmail{cfg: cfg}
}

// Name returns the source name.
func (g *Gmail) Name() string {
	return "gmail"
}

// Status returns the current state of the Gmail source.
func (g *Gmail) Status(ctx context.Context, db *store.DB) (*source.Status, error) {
	tokenStore, err := NewTokenStore(g.cfg.TokenDBPath)
	if err != nil {
		return &source.Status{Connected: false}, nil
	}
	defer tokenStore.Close()

	accounts, err := tokenStore.ListAccounts()
	if err != nil {
		return &source.Status{Connected: false}, nil
	}

	count, _ := CountEmails(db, "")
	lastSync, _ := LastSyncTime(db)

	return &source.Status{
		Connected:    len(accounts) > 0,
		Accounts:     accounts,
		ItemCount:    count,
		LastSyncedAt: lastSync,
	}, nil
}

// Login performs OAuth2 authentication for the given email account.
func (g *Gmail) Login(ctx context.Context, email string) error {
	tokenStore, err := NewTokenStore(g.cfg.TokenDBPath)
	if err != nil {
		return fmt.Errorf("open token store: %w", err)
	}
	defer tokenStore.Close()

	return Login(ctx, g.cfg.CredentialsFile, email, tokenStore)
}

// Sync fetches emails from Gmail and stores them in the database.
func (g *Gmail) Sync(ctx context.Context, db *store.DB, opts SyncOptions) (*SyncResult, error) {
	if err := Migrate(db); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	tokenStore, err := NewTokenStore(g.cfg.TokenDBPath)
	if err != nil {
		return nil, fmt.Errorf("open token store: %w", err)
	}
	defer tokenStore.Close()

	accounts, err := tokenStore.ListAccounts()
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("no authenticated accounts; run 'obk gmail auth login' first")
	}

	// Filter to single account if specified.
	if opts.Account != "" {
		found := false
		for _, a := range accounts {
			if a == opts.Account {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("account %q not authenticated", opts.Account)
		}
		accounts = []string{opts.Account}
	}

	limiter := NewRateLimiter()
	result := &SyncResult{}

	for _, email := range accounts {
		srv, err := authenticate(ctx, g.cfg.CredentialsFile, email, tokenStore)
		if err != nil {
			log.Printf("Error authenticating %s: %v", email, err)
			result.Errors++
			continue
		}

		query := FetchQuery{After: opts.After}
		msgIDs, err := SearchIDs(srv, query, limiter)
		if err != nil {
			log.Printf("Error searching %s: %v", email, err)
			result.Errors++
			continue
		}

		fmt.Printf("Found %d messages for %s\n", len(msgIDs), email)

		for _, id := range msgIDs {
			if !opts.Full {
				exists, err := EmailExists(db, id, email)
				if err != nil {
					log.Printf("Error checking email %s: %v", id, err)
					result.Errors++
					continue
				}
				if exists {
					result.Skipped++
					continue
				}
			}

			fetched, err := FetchEmail(srv, email, id, limiter)
			if err != nil {
				log.Printf("Error fetching email %s: %v", id, err)
				result.Errors++
				continue
			}

			if opts.DownloadAttachments && opts.AttachmentsDir != "" {
				if err := SaveAttachments(fetched, opts.AttachmentsDir); err != nil {
					log.Printf("Error saving attachments for %s: %v", id, err)
				}
			}

			if _, err := SaveEmail(db, fetched); err != nil {
				log.Printf("Error saving email %s: %v", id, err)
				result.Errors++
				continue
			}

			result.Fetched++
		}
	}

	return result, nil
}
