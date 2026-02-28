package gmail

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SaveAttachments saves all attachments from an email to disk.
// Files are stored under baseDir/<account>/<message-id>/.
func SaveAttachments(email *Email, baseDir string) error {
	if len(email.Attachments) == 0 {
		return nil
	}

	dir := filepath.Join(baseDir, sanitizePath(email.Account), email.MessageID)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create attachment dir %s: %w", dir, err)
	}

	for i := range email.Attachments {
		att := &email.Attachments[i]
		if len(att.Data) == 0 {
			continue
		}

		safeFilename := sanitizePath(filepath.Base(att.Filename))
		savePath := filepath.Join(dir, safeFilename)

		// Avoid overwriting if file exists.
		if _, err := os.Stat(savePath); err == nil {
			ext := filepath.Ext(safeFilename)
			base := strings.TrimSuffix(safeFilename, ext)
			savePath = filepath.Join(dir, base+"_"+email.MessageID+ext)
		}

		if err := os.WriteFile(savePath, att.Data, 0644); err != nil {
			return fmt.Errorf("write attachment %s: %w", att.Filename, err)
		}
		att.SavedPath = savePath
	}
	return nil
}

func sanitizePath(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, " ", "_")
	return s
}
