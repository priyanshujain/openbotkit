package telegram

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/73ai/openbotkit/channel/tghtml"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

// botSender abstracts the Telegram bot API for testing.
type botSender interface {
	Send(c tgbotapi.Chattable) (tgbotapi.Message, error)
	Request(c tgbotapi.Chattable) (*tgbotapi.APIResponse, error)
	MakeRequest(endpoint string, params tgbotapi.Params) (*tgbotapi.APIResponse, error)
}

type approvalResponse struct {
	approved bool
	err      error
}

// incomingMessage carries both the text and the Telegram message ID.
type incomingMessage struct {
	text      string
	messageID int
}

// callbackData carries a parsed callback from the Telegram inline keyboard.
type callbackData struct {
	ID   string // callback query ID (for answering)
	Data string // raw callback data string
}

// Channel implements channel.Channel for Telegram.
type Channel struct {
	bot      botSender
	chatID   int64
	incoming chan incomingMessage
	done     chan struct{}

	approvalMu    sync.Mutex
	approvalCh    chan approvalResponse
	approvalMsgID int

	interruptCh chan callbackData
	killTaskCh  chan callbackData
}

func NewChannel(bot botSender, chatID int64) *Channel {
	return &Channel{
		bot:         bot,
		chatID:      chatID,
		incoming:    make(chan incomingMessage, 16),
		done:        make(chan struct{}),
		interruptCh: make(chan callbackData, 1),
		killTaskCh:  make(chan callbackData, 1),
	}
}

func (c *Channel) ChatID() int64 { return c.chatID }

func (c *Channel) Send(msg string) error {
	html := tghtml.Convert(msg)
	m := tgbotapi.NewMessage(c.chatID, html)
	m.ParseMode = "HTML"
	_, err := c.bot.Send(m)
	if isTelegramBadRequest(err) {
		m.Text = msg
		m.ParseMode = ""
		_, err = c.bot.Send(m)
	}
	return err
}

func (c *Channel) Receive() (string, error) {
	msg, ok := <-c.incoming
	if !ok {
		return "", io.EOF
	}
	return msg.text, nil
}

// ReceiveMessage returns the next incoming message with its Telegram message ID.
func (c *Channel) ReceiveMessage() (incomingMessage, error) {
	msg, ok := <-c.incoming
	if !ok {
		return incomingMessage{}, io.EOF
	}
	return msg, nil
}

func (c *Channel) SendLink(text string, url string) error {
	if strings.Contains(url, "/auth/redirect") {
		return c.sendWebAppLink(text, url)
	}
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL(text, url),
		),
	)
	msg := tgbotapi.NewMessage(c.chatID, text)
	msg.ReplyMarkup = keyboard
	_, err := c.bot.Send(msg)
	return err
}

// webAppInfo mirrors Telegram's WebAppInfo for the inline keyboard web_app field.
type webAppInfo struct {
	URL string `json:"url"`
}

type webAppButton struct {
	Text   string     `json:"text"`
	WebApp webAppInfo `json:"web_app"`
}

type webAppKeyboard struct {
	InlineKeyboard [][]webAppButton `json:"inline_keyboard"`
}

// sendWebAppLink sends a Mini App button so the trampoline page
// can use Telegram.WebApp.openLink() to open the system browser.
func (c *Channel) sendWebAppLink(text string, url string) error {
	msg := tgbotapi.NewMessage(c.chatID, text)
	msg.ReplyMarkup = webAppKeyboard{
		InlineKeyboard: [][]webAppButton{{
			{Text: text, WebApp: webAppInfo{URL: url}},
		}},
	}
	_, err := c.bot.Send(msg)
	return err
}

func (c *Channel) RequestApproval(action string) (bool, error) {
	c.approvalMu.Lock()
	c.approvalCh = make(chan approvalResponse, 1)
	c.approvalMu.Unlock()

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("Approve", "approve"),
			tgbotapi.NewInlineKeyboardButtonData("Deny", "deny"),
		),
	)

	msg := tgbotapi.NewMessage(c.chatID, fmt.Sprintf("Approve action?\n\n%s", action))
	msg.ReplyMarkup = keyboard
	sentMsg, err := c.bot.Send(msg)
	if err != nil {
		return false, fmt.Errorf("send approval request: %w", err)
	}
	c.approvalMu.Lock()
	c.approvalMsgID = sentMsg.MessageID
	ch := c.approvalCh
	c.approvalMu.Unlock()

	resp := <-ch
	return resp.approved, resp.err
}

// HandleCallback processes an inline keyboard callback, routing by data prefix.
func (c *Channel) HandleCallback(callbackID string, data string) {
	switch {
	case strings.HasPrefix(data, "interrupt:"):
		c.interruptCh <- callbackData{ID: callbackID, Data: data}
		return
	case strings.HasPrefix(data, "kill_task:"):
		c.killTaskCh <- callbackData{ID: callbackID, Data: data}
		return
	}

	// Default: approval flow.
	c.approvalMu.Lock()
	ch := c.approvalCh
	msgID := c.approvalMsgID
	c.approvalMsgID = 0
	c.approvalMu.Unlock()

	approved := data == "approve"

	label := "Approved"
	if !approved {
		label = "Denied"
	}
	answer := tgbotapi.NewCallback(callbackID, label)
	c.bot.Request(answer)

	if msgID != 0 {
		edit := tgbotapi.NewEditMessageReplyMarkup(
			c.chatID, msgID,
			tgbotapi.InlineKeyboardMarkup{InlineKeyboard: [][]tgbotapi.InlineKeyboardButton{}},
		)
		c.bot.Request(edit)
	}

	if ch != nil {
		ch <- approvalResponse{approved: approved}
	}
}

// CancelPendingApproval sends a denial to unblock RequestApproval if the
// agent is killed while waiting for user approval.
func (c *Channel) CancelPendingApproval() {
	c.approvalMu.Lock()
	ch := c.approvalCh
	c.approvalCh = nil
	c.approvalMu.Unlock()
	if ch != nil {
		ch <- approvalResponse{approved: false}
	}
}

// PushMessage enqueues an incoming message from the poller.
func (c *Channel) PushMessage(text string, messageID int) {
	c.incoming <- incomingMessage{text: text, messageID: messageID}
}

// isTelegramBadRequest returns true if the error is a Telegram API 400 error
// (e.g. HTML parse failure). Other errors (network, rate limit) are not retried
// to avoid sending duplicate messages.
func isTelegramBadRequest(err error) bool {
	var apiErr *tgbotapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == 400
}

// Close shuts down the incoming channel.
func (c *Channel) Close() {
	close(c.incoming)
}
