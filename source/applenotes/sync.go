package applenotes

import (
	"fmt"
	"log"

	"github.com/priyanshujain/openbotkit/store"
)

func Sync(db *store.DB, opts SyncOptions) (*SyncResult, error) {
	if err := Migrate(db); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	log.Println("applenotes: fetching folders...")
	folders, noteToFolder, err := FetchFolders()
	if err != nil {
		return nil, fmt.Errorf("fetch folders: %w", err)
	}
	log.Printf("applenotes: found %d folders", len(folders))

	for i := range folders {
		if err := SaveFolder(db, &folders[i]); err != nil {
			log.Printf("applenotes: save folder %q: %v", folders[i].Name, err)
		}
	}

	log.Println("applenotes: fetching notes...")
	notes, err := FetchAllNotes()
	if err != nil {
		return nil, fmt.Errorf("fetch notes: %w", err)
	}
	log.Printf("applenotes: found %d notes", len(notes))

	result := &SyncResult{}
	for i := range notes {
		n := &notes[i]

		// Attach folder info from the folder map
		if f, ok := noteToFolder[n.AppleID]; ok {
			n.Folder = f.Name
			n.FolderID = f.AppleID
			n.Account = f.Account
		}

		// Skip password-protected notes body (metadata is still saved)
		if n.PasswordProtected {
			n.Body = ""
		}

		if err := SaveNote(db, n); err != nil {
			log.Printf("applenotes: save note %q: %v", n.Title, err)
			result.Errors++
			continue
		}
		result.Synced++
	}

	return result, nil
}
