package telegram

import (
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/73ai/openbotkit/config"
	tgsrc "github.com/73ai/openbotkit/source/telegram"
	"github.com/spf13/cobra"
)

var messagesCmd = &cobra.Command{
	Use:   "messages",
	Short: "Query stored Telegram messages",
}

var messagesListCmd = &cobra.Command{
	Use:   "list",
	Short: "List stored messages with optional filters",
	Example: `  obk telegram messages list
  obk telegram messages list --chat -1001234567890 --limit 20
  obk telegram messages list --after 2026-01-01 --before 2026-02-01 --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		db, err := openTelegramDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		chat, _ := cmd.Flags().GetInt64("chat")
		after, _ := cmd.Flags().GetString("after")
		before, _ := cmd.Flags().GetString("before")
		limit, _ := cmd.Flags().GetInt("limit")
		jsonOut, _ := cmd.Flags().GetBool("json")

		messages, err := tgsrc.ListMessages(db, tgsrc.ListOptions{
			ChatID: chat,
			After:  after,
			Before: before,
			Limit:  limit,
		})
		if err != nil {
			return fmt.Errorf("list messages: %w", err)
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(messages)
		}
		return printMessages(messages)
	},
}

var messagesSearchCmd = &cobra.Command{
	Use:   "search <query>",
	Short: "Full-text search across message text",
	Args:  cobra.ExactArgs(1),
	Example: `  obk telegram messages search "meeting tomorrow"
  obk telegram messages search "invoice" --limit 10 --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		db, err := openTelegramDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		jsonOut, _ := cmd.Flags().GetBool("json")
		limit, _ := cmd.Flags().GetInt("limit")

		messages, err := tgsrc.SearchMessages(db, args[0], limit)
		if err != nil {
			return fmt.Errorf("search messages: %w", err)
		}

		if jsonOut {
			return json.NewEncoder(os.Stdout).Encode(messages)
		}
		return printMessages(messages)
	},
}

func printMessages(messages []tgsrc.Message) error {
	if len(messages) == 0 {
		fmt.Println("No messages found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIMESTAMP\tCHAT\tSENDER\tTEXT")
	for _, m := range messages {
		sender := m.SenderName
		if sender == "" && m.SenderID != 0 {
			sender = fmt.Sprintf("%d", m.SenderID)
		}
		fmt.Fprintf(w, "%s\t%d\t%s\t%s\n",
			m.Timestamp.Format("2006-01-02 15:04"), m.ChatID, truncate(sender, 30), truncate(m.Text, 60))
	}
	return w.Flush()
}

func truncate(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max-3]) + "..."
}

var chatsCmd = &cobra.Command{
	Use:   "chats",
	Short: "Query stored Telegram chats",
}

var chatsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all synced chats",
	Example: `  obk telegram chats list
  obk telegram chats list --json`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		db, err := openTelegramDB(cfg)
		if err != nil {
			return err
		}
		defer db.Close()

		jsonOut, _ := cmd.Flags().GetBool("json")
		query, _ := cmd.Flags().GetString("search")

		var chats []tgsrc.Chat
		if query != "" {
			chats, err = tgsrc.FindChats(db, query, 0)
		} else {
			chats, err = tgsrc.ListChats(db)
		}
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
		fmt.Fprintln(w, "CHAT ID\tTYPE\tTITLE\tUSERNAME\tLAST MESSAGE")
		for _, c := range chats {
			lastMsg := "never"
			if c.LastMessageAt != nil {
				lastMsg = c.LastMessageAt.Format("2006-01-02 15:04")
			}
			username := c.Username
			if username != "" {
				username = "@" + username
			}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", c.ChatID, c.Type, truncate(c.Title, 30), username, lastMsg)
		}
		return w.Flush()
	},
}

func init() {
	messagesListCmd.Flags().Int64("chat", 0, "Filter by chat ID")
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
	chatsListCmd.Flags().String("search", "", "Filter chats by title or username")
	chatsCmd.AddCommand(chatsListCmd)
}
