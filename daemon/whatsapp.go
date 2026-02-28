package daemon

import (
	"context"
	"log"

	"github.com/priyanshujain/openbotkit/config"
	wasrc "github.com/priyanshujain/openbotkit/source/whatsapp"
	"github.com/priyanshujain/openbotkit/store"
)

// runWhatsAppSync starts a WhatsApp sync goroutine that runs until ctx is cancelled.
// Errors are sent on the returned channel (non-blocking).
func runWhatsAppSync(ctx context.Context, cfg *config.Config) <-chan error {
	errCh := make(chan error, 1)

	go func() {
		defer close(errCh)

		client, err := wasrc.NewClient(ctx, cfg.WhatsAppSessionDBPath())
		if err != nil {
			log.Printf("whatsapp: failed to create client: %v", err)
			errCh <- err
			return
		}

		if !client.IsAuthenticated() {
			log.Println("whatsapp: not authenticated, skipping sync (run 'obk whatsapp auth' first)")
			return
		}

		db, err := store.Open(store.Config{
			Driver: cfg.WhatsApp.Storage.Driver,
			DSN:    cfg.WhatsAppDataDSN(),
		})
		if err != nil {
			log.Printf("whatsapp: failed to open db: %v", err)
			errCh <- err
			return
		}
		defer db.Close()

		log.Println("whatsapp: starting sync")
		result, err := wasrc.Sync(ctx, client, db, wasrc.SyncOptions{
			Follow: true,
		})
		if err != nil {
			log.Printf("whatsapp: sync error: %v", err)
			errCh <- err
			return
		}

		log.Printf("whatsapp: sync stopped: received=%d history=%d errors=%d",
			result.Received, result.HistoryMessages, result.Errors)
	}()

	return errCh
}
