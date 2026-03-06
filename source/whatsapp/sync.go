package whatsapp

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync/atomic"
	"time"

	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/proto/waWeb"
	"go.mau.fi/whatsmeow/types/events"

	"github.com/priyanshujain/openbotkit/store"
)

func Sync(ctx context.Context, client *Client, db *store.DB, opts SyncOptions) (*SyncResult, error) {
	if err := Migrate(db); err != nil {
		return nil, fmt.Errorf("migrate schema: %w", err)
	}

	var received atomic.Int64
	var historyMsgs atomic.Int64
	var errors atomic.Int64

	client.WM().AddEventHandler(func(rawEvt any) {
		switch evt := rawEvt.(type) {
		case *events.Message:
			msg := parseMessage(evt)
			if msg == nil {
				return
			}
			if err := SaveMessage(db, msg); err != nil {
				log.Printf("save message: %v", err)
				errors.Add(1)
				return
			}
			UpsertChat(db, msg.ChatJID, chatName(evt), msg.IsGroup)
			received.Add(1)
			log.Printf("message from %s in %s: %s", msg.SenderName, msg.ChatJID, truncate(msg.Text, 80))

		case *events.HistorySync:
			for _, conv := range evt.Data.GetConversations() {
				chatJID := conv.GetID()
				chatDisplayName := conv.GetDisplayName()
				if chatDisplayName == "" {
					chatDisplayName = conv.GetName()
				}
				isGroup := strings.HasSuffix(chatJID, "@g.us")
				UpsertChat(db, chatJID, chatDisplayName, isGroup)

				for _, hMsg := range conv.GetMessages() {
					msg := parseHistoryMessage(hMsg.GetMessage(), chatJID, isGroup)
					if msg == nil {
						continue
					}
					if err := SaveMessage(db, msg); err != nil {
						errors.Add(1)
						continue
					}
					historyMsgs.Add(1)
				}
			}
			log.Printf("history sync: %d conversations processed", len(evt.Data.GetConversations()))

		case *events.Connected:
			log.Println("connected to whatsapp")

		case *events.Disconnected:
			log.Println("disconnected from whatsapp")
			if opts.Follow {
				go func() {
					if err := client.ReconnectWithBackoff(ctx); err != nil {
						log.Printf("reconnect failed: %v", err)
					}
				}()
			}
		}
	})

	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}

	go syncContacts(ctx, client, db)

	if opts.Follow {
		<-ctx.Done()
	} else {
		time.Sleep(30 * time.Second)
	}

	client.Disconnect()

	return &SyncResult{
		Received:        int(received.Load()),
		HistoryMessages: int(historyMsgs.Load()),
		Errors:          int(errors.Load()),
	}, nil
}

func syncContacts(ctx context.Context, client *Client, db *store.DB) {
	// Wait a bit for the connection to stabilize and contacts to load.
	time.Sleep(5 * time.Second)

	contacts, err := client.GetAllContacts(ctx)
	if err != nil {
		log.Printf("sync contacts: %v", err)
		return
	}

	var saved int
	for _, c := range contacts {
		if err := SaveContact(db, &c); err != nil {
			log.Printf("save contact %s: %v", c.JID, err)
			continue
		}
		saved++
	}
	log.Printf("contacts synced: %d/%d", saved, len(contacts))
}

func parseMessage(evt *events.Message) *Message {
	if evt.Message == nil {
		return nil
	}

	text, mediaType := extractContent(evt.Message)
	if text == "" && mediaType == "" {
		return nil
	}

	var replyToID string
	if ci := getContextInfo(evt.Message); ci != nil {
		replyToID = ci.GetStanzaID()
	}

	return &Message{
		MessageID:  evt.Info.ID,
		ChatJID:    evt.Info.Chat.String(),
		SenderJID:  evt.Info.Sender.String(),
		SenderName: evt.Info.PushName,
		Text:       text,
		Timestamp:  evt.Info.Timestamp,
		MediaType:  mediaType,
		IsGroup:    evt.Info.IsGroup,
		IsFromMe:   evt.Info.IsFromMe,
		ReplyToID:  replyToID,
	}
}

func parseHistoryMessage(webMsg *waWeb.WebMessageInfo, chatJID string, isGroup bool) *Message {
	if webMsg == nil || webMsg.GetMessage() == nil {
		return nil
	}

	key := webMsg.GetKey()
	text, mediaType := extractContent(webMsg.GetMessage())
	if text == "" && mediaType == "" {
		return nil
	}

	senderJID := key.GetParticipant()
	if senderJID == "" {
		senderJID = key.GetRemoteJID()
	}

	ts := time.Unix(int64(webMsg.GetMessageTimestamp()), 0)

	return &Message{
		MessageID:  key.GetID(),
		ChatJID:    chatJID,
		SenderJID:  senderJID,
		SenderName: webMsg.GetPushName(),
		Text:       text,
		Timestamp:  ts,
		MediaType:  mediaType,
		IsGroup:    isGroup,
		IsFromMe:   key.GetFromMe(),
	}
}

func extractContent(msg *waE2E.Message) (text string, mediaType string) {
	if msg == nil {
		return "", ""
	}

	if t := msg.GetConversation(); t != "" {
		return t, ""
	}
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetText(), ""
	}
	if img := msg.GetImageMessage(); img != nil {
		return img.GetCaption(), "image"
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		return vid.GetCaption(), "video"
	}
	if aud := msg.GetAudioMessage(); aud != nil {
		return "", "audio"
	}
	if doc := msg.GetDocumentMessage(); doc != nil {
		return doc.GetCaption(), "document"
	}
	return "", ""
}

func getContextInfo(msg *waE2E.Message) *waE2E.ContextInfo {
	if ext := msg.GetExtendedTextMessage(); ext != nil {
		return ext.GetContextInfo()
	}
	if img := msg.GetImageMessage(); img != nil {
		return img.GetContextInfo()
	}
	if vid := msg.GetVideoMessage(); vid != nil {
		return vid.GetContextInfo()
	}
	return nil
}

func chatName(evt *events.Message) string {
	if evt.Info.PushName != "" && !evt.Info.IsGroup {
		return evt.Info.PushName
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

