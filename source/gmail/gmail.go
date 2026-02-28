package gmail

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/priyanshujain/openbotkit/source"
	"github.com/priyanshujain/openbotkit/store"
	gapi "google.golang.org/api/gmail/v1"
)

type Gmail struct {
	cfg Config
}

func New(cfg Config) *Gmail {
	return &Gmail{cfg: cfg}
}

func (g *Gmail) Name() string {
	return "gmail"
}

func (g *Gmail) Status(ctx context.Context, db *store.DB) (*source.Status, error) {
	accounts, err := g.cfg.Provider.Accounts(ctx)
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

func (g *Gmail) resolveAccount(ctx context.Context, account string) (string, error) {
	accounts, err := g.cfg.Provider.Accounts(ctx)
	if err != nil {
		return "", fmt.Errorf("list accounts: %w", err)
	}
	if len(accounts) == 0 {
		return "", fmt.Errorf("no authenticated accounts; run 'obk auth google login' first")
	}

	if account != "" {
		for _, a := range accounts {
			if a == account {
				return account, nil
			}
		}
		return "", fmt.Errorf("account %q not authenticated", account)
	}

	if len(accounts) == 1 {
		return accounts[0], nil
	}
	return "", fmt.Errorf("multiple accounts found; specify one with --account")
}

func (g *Gmail) Send(ctx context.Context, input ComposeInput) (*SendResult, error) {
	account, err := g.resolveAccount(ctx, input.Account)
	if err != nil {
		return nil, err
	}
	input.Account = account

	httpClient, err := g.cfg.Provider.Client(ctx, account, []string{gapi.GmailComposeScope})
	if err != nil {
		return nil, fmt.Errorf("get client for %s: %w", account, err)
	}

	srv, err := newGmailService(ctx, httpClient)
	if err != nil {
		return nil, err
	}

	return SendEmail(srv, input, NewRateLimiter())
}

func (g *Gmail) CreateDraft(ctx context.Context, input ComposeInput) (*DraftResult, error) {
	account, err := g.resolveAccount(ctx, input.Account)
	if err != nil {
		return nil, err
	}
	input.Account = account

	httpClient, err := g.cfg.Provider.Client(ctx, account, []string{gapi.GmailComposeScope})
	if err != nil {
		return nil, fmt.Errorf("get client for %s: %w", account, err)
	}

	srv, err := newGmailService(ctx, httpClient)
	if err != nil {
		return nil, err
	}

	return CreateDraft(srv, input, NewRateLimiter())
}

func (g *Gmail) Sync(ctx context.Context, db *store.DB, opts SyncOptions) (*SyncResult, error) {
	if err := Migrate(db); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	// Default to 7-day window unless full sync or explicit date.
	if opts.After == "" && !opts.Full {
		days := opts.DaysWindow
		if days == 0 {
			days = 7
		}
		opts.After = time.Now().AddDate(0, 0, -days).Format("2006/01/02")
	}

	accounts, err := g.cfg.Provider.Accounts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list accounts: %w", err)
	}
	if len(accounts) == 0 {
		return nil, fmt.Errorf("no authenticated accounts; run 'obk auth google login' first")
	}

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
		httpClient, err := g.cfg.Provider.Client(ctx, email, []string{gapi.GmailReadonlyScope})
		if err != nil {
			log.Printf("Error getting client for %s: %v", email, err)
			result.Errors++
			continue
		}

		srv, err := newGmailService(ctx, httpClient)
		if err != nil {
			log.Printf("Error creating service for %s: %v", email, err)
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
