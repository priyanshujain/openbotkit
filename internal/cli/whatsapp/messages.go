package whatsapp

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/priyanshujain/openbotkit/config"
	wasrc "github.com/priyanshujain/openbotkit/source/whatsapp"
	"github.com/priyanshujain/openbotkit/store"
	"github.com/spf13/cobra"
)

var messagesCmd = &cobra.Command{
	Use:   "messages",
	Short: "Query stored WhatsApp messages",
}

var messagesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored messages with optional filters",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		db, err := openWhatsAppDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		chat, _ := cmd.Flags().GetString("chat")
		after, _ := cmd.Flags().GetString("after")
		before, _ := cmd.Flags().GetString("before")
		limit, _ := cmd.Flags().GetInt("limit")
		jsonOut, _ := cmd.Flags().GetBool("json")

		messages, err := wasrc.ListMessages(db, wasrc.ListOptions{
			ChatJID: chat,
			After:   after,
			Before:  before,
			Limit:   limit,
		})
		if err != nil {
			return fmt.Errorf("list messages: %w", err)
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(messages)
		}

		if len(messages) == 0 {
			fmt.Println("No messages found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TIMESTAMP\tCHAT\tSENDER\tTEXT")
		for _, m := range messages {
			text := m.Text
			if len(text) > 60 {
				text = text[:57] + "..."
			}
			sender := m.SenderName
			if sender == "" {
				sender = m.SenderJID
			}
			if len(sender) > 30 {
				sender = sender[:27] + "..."
			}
			chat := m.ChatJID
			if len(chat) > 30 {
				chat = chat[:27] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				m.Timestamp.Format("2006-01-02 15:04"), chat, sender, text)
		}
		return w.Flush()
	},
}

var messagesSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search across message text",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		db, err := openWhatsAppDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		jsonOut, _ := cmd.Flags().GetBool("json")
		limit, _ := cmd.Flags().GetInt("limit")

		messages, err := wasrc.SearchMessages(db, args[0], limit)
		if err != nil {
			return fmt.Errorf("search messages: %w", err)
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(messages)
		}

		if len(messages) == 0 {
			fmt.Println("No messages found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "TIMESTAMP\tCHAT\tSENDER\tTEXT")
		for _, m := range messages {
			text := m.Text
			if len(text) > 60 {
				text = text[:57] + "..."
			}
			sender := m.SenderName
			if sender == "" {
				sender = m.SenderJID
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
				m.Timestamp.Format("2006-01-02 15:04"), m.ChatJID, sender, text)
		}
		return w.Flush()
	},
}

var chatsCmd = &cobra.Command{
	Use:   "chats",
	Short: "Query stored WhatsApp chats",
}

var chatsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all synced chats with message counts",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		db, err := openWhatsAppDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		jsonOut, _ := cmd.Flags().GetBool("json")

		chats, err := wasrc.ListChats(db)
		if err != nil {
			return fmt.Errorf("list chats: %w", err)
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(chats)
		}

		if len(chats) == 0 {
			fmt.Println("No chats found.")
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
		fmt.Fprintln(w, "JID\tNAME\tGROUP\tLAST MESSAGE")
		for _, c := range chats {
			group := "no"
			if c.IsGroup {
				group = "yes"
			}
			lastMsg := "never"
			if c.LastMessageAt != nil {
				lastMsg = c.LastMessageAt.Format("2006-01-02 15:04")
			}
			name := c.Name
			if len(name) > 30 {
				name = name[:27] + "..."
			}
			jid := c.JID
			if len(jid) > 30 {
				jid = jid[:27] + "..."
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", jid, name, group, lastMsg)
		}
		return w.Flush()
	},
}

func init() {
	messagesListCmd.Flags().String("chat", "", "Filter by chat JID")
	messagesListCmd.Flags().String("after", "", "Messages after date (YYYY-MM-DD)")
	messagesListCmd.Flags().String("before", "", "Messages before date (YYYY-MM-DD)")
	messagesListCmd.Flags().Int("limit", 50, "Maximum number of results")
	messagesListCmd.Flags().Bool("json", false, "Output as JSON")

	messagesSearchCmd.Flags().Bool("json", false, "Output as JSON")
	messagesSearchCmd.Flags().Int("limit", 50, "Maximum number of results")

	messagesCmd.AddCommand(messagesListCmd)
	messagesCmd.AddCommand(messagesSearchCmd)
	messagesCmd.AddCommand(messagesSendCmd)

	chatsListCmd.Flags().Bool("json", false, "Output as JSON")
	chatsCmd.AddCommand(chatsListCmd)
}

func openWhatsAppDB(cfg *config.Config) (*store.DB, error) {
	if err := config.EnsureSourceDir("whatsapp"); err != nil {
		return nil, fmt.Errorf("create whatsapp dir: %w", err)
	}

	dsn := cfg.WhatsAppDataDSN()
	db, err := store.Open(store.Config{
		Driver: cfg.WhatsApp.Storage.Driver,
		DSN:    dsn,
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := wasrc.Migrate(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	return db, nil
}
