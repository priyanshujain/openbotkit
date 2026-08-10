package channel

import (
	"context"
	"errors"
	"fmt"

	"github.com/73ai/openbotkit/channel/tghtml"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type TelegramPusher struct {
	bot    *tgbotapi.BotAPI
	chatID int64
}

var _ Pusher = (*TelegramPusher)(nil)

func NewTelegramPusher(botToken string, chatID int64) (*TelegramPusher, error) {
	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		return nil, fmt.Errorf("create telegram bot: %w", err)
	}
	return &TelegramPusher{bot: bot, chatID: chatID}, nil
}

func (p *TelegramPusher) Push(_ context.Context, message string) error {
	html := tghtml.Convert(message)
	m := tgbotapi.NewMessage(p.chatID, html)
	m.ParseMode = "HTML"
	_, err := p.bot.Send(m)
	if isTGBadRequest(err) {
		m.Text = message
		m.ParseMode = ""
		_, err = p.bot.Send(m)
	}
	return err
}

func isTGBadRequest(err error) bool {
	var apiErr *tgbotapi.Error
	return errors.As(err, &apiErr) && apiErr.Code == 400
}
