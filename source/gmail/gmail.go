package gmail

import (
	"context"
	"fmt"
	"log"

	"github.com/priyanshujain/openbotkit/source"
	"github.com/priyanshujain/openbotkit/store"
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

func (g *Gmail) Login(ctx context.Context, email string) error {
	tokenStore, err := NewTokenStore(g.cfg.TokenDBPath)
	if err != nil {
		return fmt.Errorf("open token store: %w", err)
	}
	defer tokenStore.Close()

	return Login(ctx, g.cfg.CredentialsFile, email, tokenStore)
}

func (g *Gmail) resolveAccount(tokenStore *TokenStore, account string) (string, error) {
	accounts, err := tokenStore.ListAccounts()
	if err != nil {
		return "", fmt.Errorf("list accounts: %w", err)
	}
	if len(accounts) == 0 {
		return "", fmt.Errorf("no authenticated accounts; run 'obk gmail auth login' first")
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
	tokenStore, err := NewTokenStore(g.cfg.TokenDBPath)
	if err != nil {
		return nil, fmt.Errorf("open token store: %w", err)
	}
	defer tokenStore.Close()

	account, err := g.resolveAccount(tokenStore, input.Account)
	if err != nil {
		return nil, err
	}
	input.Account = account

	srv, err := authenticate(ctx, g.cfg.CredentialsFile, account, tokenStore)
	if err != nil {
		return nil, fmt.Errorf("authenticate %s: %w", account, err)
	}

	return SendEmail(srv, input, NewRateLimiter())
}

func (g *Gmail) CreateDraft(ctx context.Context, input ComposeInput) (*DraftResult, error) {
	tokenStore, err := NewTokenStore(g.cfg.TokenDBPath)
	if err != nil {
		return nil, fmt.Errorf("open token store: %w", err)
	}
	defer tokenStore.Close()

	account, err := g.resolveAccount(tokenStore, input.Account)
	if err != nil {
		return nil, err
	}
	input.Account = account

	srv, err := authenticate(ctx, g.cfg.CredentialsFile, account, tokenStore)
	if err != nil {
		return nil, fmt.Errorf("authenticate %s: %w", account, err)
	}

	return CreateDraft(srv, input, NewRateLimiter())
}

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
