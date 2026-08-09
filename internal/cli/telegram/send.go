package telegram

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/73ai/openbotkit/config"
	tgsrc "github.com/73ai/openbotkit/source/telegram"
	"github.com/73ai/openbotkit/store"
	"github.com/spf13/cobra"
)

var messagesSendCmd = &cobra.Command{
	Use:   "send",
	Short: "Send a text message to a Telegram chat",
	Example: `  obk telegram messages send --to -1001234567890 --text "Hello!"
  obk telegram messages send --to @ann --text "Hello!"`,
	RunE: func(cmd *cobra.Command, args []string) error {
		to, _ := cmd.Flags().GetString("to")
		text, _ := cmd.Flags().GetString("text")

		if to == "" {
			return fmt.Errorf("--to flag is required")
		}
		if text == "" {
			return fmt.Errorf("--text flag is required")
		}

		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}
		if err := requireSession(cfg); err != nil {
			return err
		}

		db, err := openTelegramDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		chatID, err := resolveChatID(db, to)
		if err != nil {
			return err
		}

		client, err := newClient(cfg, db)
		if err != nil {
			return err
		}

		var result *tgsrc.SendResult
		err = client.Run(cmd.Context(), func(ctx context.Context) error {
			result, err = tgsrc.SendText(ctx, client.API(), db, tgsrc.SendInput{
				ChatID: chatID,
				Text:   text,
			})
			return err
		})
		if err != nil {
			return fmt.Errorf("send message: %w", err)
		}

		fmt.Printf("Message sent: id=%d chat=%d at %s\n",
			result.MessageID, chatID, result.Timestamp.Format("2006-01-02 15:04:05"))
		return nil
	},
}

// resolveChatID accepts a numeric chat ID or a @username / title fragment.
// Ambiguous names are reported rather than guessed.
func resolveChatID(db *store.DB, to string) (int64, error) {
	if id, err := strconv.ParseInt(to, 10, 64); err == nil {
		return id, nil
	}

	query := strings.TrimPrefix(to, "@")
	chats, err := tgsrc.FindChats(db, query, 10)
	if err != nil {
		return 0, fmt.Errorf("resolve %q: %w", to, err)
	}

	switch len(chats) {
	case 0:
		return 0, fmt.Errorf("no chat matches %q — run 'obk telegram chats list' to see what is synced", to)
	case 1:
		return chats[0].ChatID, nil
	default:
		var b strings.Builder
		fmt.Fprintf(&b, "%q matches %d chats, pass --to with one of these IDs:", to, len(chats))
		for _, c := range chats {
			fmt.Fprintf(&b, "\n  %d  %s", c.ChatID, c.Title)
		}
		return 0, fmt.Errorf("%s", b.String())
	}
}

func init() {
	messagesSendCmd.Flags().String("to", "", "Chat ID, @username or chat title (required)")
	messagesSendCmd.Flags().String("text", "", "Message text (required)")
}
