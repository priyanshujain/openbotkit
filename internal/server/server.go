package server

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"sync"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"

	"github.com/73ai/openbotkit/agent/tools"
	"github.com/73ai/openbotkit/channel"
	tgchannel "github.com/73ai/openbotkit/channel/telegram"
	wachannel "github.com/73ai/openbotkit/channel/whatsapp"
	"github.com/73ai/openbotkit/config"
	"github.com/73ai/openbotkit/internal/skills"
	"github.com/73ai/openbotkit/oauth/google"
	"github.com/73ai/openbotkit/provider"
	contactsrc "github.com/73ai/openbotkit/service/contacts"
	learningssvc "github.com/73ai/openbotkit/service/learnings"
	schedsrc "github.com/73ai/openbotkit/service/scheduler"
	ansrc "github.com/73ai/openbotkit/source/applenotes"
	gmailsrc "github.com/73ai/openbotkit/source/gmail"
	imsrc "github.com/73ai/openbotkit/source/imessage"
	wasrc "github.com/73ai/openbotkit/source/whatsapp"
	"github.com/73ai/openbotkit/store"

	// Register provider factories.
	_ "github.com/73ai/openbotkit/provider/anthropic"
	_ "github.com/73ai/openbotkit/provider/gemini"
	_ "github.com/73ai/openbotkit/provider/groq"
	_ "github.com/73ai/openbotkit/provider/openai"
	_ "github.com/73ai/openbotkit/provider/openrouter"
)

type Server struct {
	cfg  *config.Config
	addr string
	ctx  context.Context

	waMu   sync.Mutex
	waAuth *whatsAppAuth

	scopeWaiter *google.ScopeWaiter
	google      *google.Google
	learnings   *learningssvc.Store
	credTokens  *credentialTokenStore
}

func New(cfg *config.Config, addr string) *Server {
	return &Server{cfg: cfg, addr: addr, credTokens: newCredentialTokenStore()}
}

func (s *Server) Run(ctx context.Context) error {
	s.ctx = ctx

	u, p := s.authCredentials()
	if u == "" || p == "" {
		return fmt.Errorf("server requires authentication credentials; set OBK_AUTH_USERNAME and OBK_AUTH_PASSWORD env vars or configure auth in config.yaml")
	}

	s.scopeWaiter = google.NewScopeWaiter()
	s.google = google.New(google.Config{
		CredentialsFile: s.cfg.GoogleCredentialsFile(),
		TokenDBPath:     s.cfg.GoogleTokenDBPath(),
		CallbackURL:     s.cfg.GWSCallbackURL(),
	})

	s.learnings = learningssvc.New(config.LearningsDir())

	s.migrateDBs()

	go func() {
		if err := skills.RefreshGWSSkills(s.cfg); err != nil {
			slog.Warn("gws skill refresh failed", "error", err)
		}
	}()

	mux := http.NewServeMux()
	s.routes(mux)

	srv := &http.Server{
		Addr:              s.addr,
		Handler:           limitBody(mux),
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(_ net.Listener) context.Context { return ctx },
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("server listening", "addr", s.addr)
		errCh <- srv.ListenAndServe()
	}()

	// Start Telegram bot if configured.
	if err := s.startTelegram(ctx); err != nil {
		slog.Warn("telegram not started", "error", err)
	}

	// Start WhatsApp channel(s) if configured.
	s.startWhatsAppChannels(ctx)

	select {
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	case <-ctx.Done():
		slog.Info("shutting down server")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}

func (s *Server) startTelegram(ctx context.Context) error {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" && s.cfg.Channels != nil && s.cfg.Channels.Telegram != nil {
		token = s.cfg.Channels.Telegram.BotToken
	}
	if token == "" {
		return fmt.Errorf("no telegram bot token configured")
	}

	var ownerID int64
	if idStr := os.Getenv("TELEGRAM_OWNER_ID"); idStr != "" {
		var err error
		ownerID, err = strconv.ParseInt(idStr, 10, 64)
		if err != nil {
			return fmt.Errorf("parse TELEGRAM_OWNER_ID: %w", err)
		}
	} else if s.cfg.Channels != nil && s.cfg.Channels.Telegram != nil {
		ownerID = s.cfg.Channels.Telegram.OwnerID
	}
	if ownerID == 0 {
		return fmt.Errorf("no telegram owner ID configured")
	}

	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return fmt.Errorf("create telegram bot: %w", err)
	}

	// Resolve the default model's provider.
	registry, err := provider.NewRegistry(s.cfg.Models)
	if err != nil {
		return fmt.Errorf("create provider registry: %w", err)
	}

	providerName, modelName, err := provider.ParseModelSpec(s.cfg.Models.Default)
	if err != nil {
		return fmt.Errorf("parse model spec: %w", err)
	}
	p, ok := registry.Get(providerName)
	if !ok {
		return fmt.Errorf("provider %q not found", providerName)
	}

	ch := tgchannel.NewChannel(bot, ownerID)

	interactor := channel.NewInteractor(ch)
	account := s.resolveAccount()
	bridge := tools.NewTokenBridge(s.google, account)

	sm := tgchannel.NewSessionManager(s.cfg, ch, p, providerName, modelName, tgchannel.SessionManagerDeps{
		Interactor:  interactor,
		ScopeWaiter: s.scopeWaiter,
		TokenBridge: bridge,
		GoogleAuth:  s.google,
		Account:     account,
	})

	poller := tgchannel.NewPoller(bot, ownerID, ch, sm)
	go poller.Run(ctx)
	go sm.Run(ctx)

	slog.Info("telegram bot started", "owner_id", ownerID)
	return nil
}

func (s *Server) startWhatsAppChannels(ctx context.Context) {
	if s.cfg.Models == nil || s.cfg.Models.Default == "" {
		return
	}

	registry, err := provider.NewRegistry(s.cfg.Models)
	if err != nil {
		slog.Warn("whatsapp channel: create provider registry", "error", err)
		return
	}

	providerName, modelName, err := provider.ParseModelSpec(s.cfg.Models.Default)
	if err != nil {
		slog.Warn("whatsapp channel: parse model spec", "error", err)
		return
	}
	p, ok := registry.Get(providerName)
	if !ok {
		slog.Warn("whatsapp channel: provider not found", "provider", providerName)
		return
	}

	for _, acct := range s.cfg.WhatsAppAccountList() {
		if acct.Role != "channel" && acct.Role != "both" {
			continue
		}
		if acct.OwnerJID == "" {
			slog.Warn("whatsapp channel: no owner_jid configured", "account", acct.Label)
			continue
		}
		if !config.IsWhatsAppAccountLinked(acct.Label) {
			slog.Info("whatsapp channel: not linked, skipping", "account", acct.Label)
			continue
		}

		sessionDBPath := s.cfg.WhatsAppAccountSessionDBPath(acct.Label)
		client, err := wasrc.NewClient(ctx, sessionDBPath)
		if err != nil {
			slog.Warn("whatsapp channel: create client", "account", acct.Label, "error", err)
			continue
		}
		if !client.IsAuthenticated() {
			slog.Warn("whatsapp channel: not authenticated", "account", acct.Label)
			continue
		}
		if err := client.Connect(ctx); err != nil {
			slog.Warn("whatsapp channel: connect", "account", acct.Label, "error", err)
			continue
		}

		db, err := store.Open(store.Config{
			Driver: s.cfg.WhatsApp.Storage.Driver,
			DSN:    s.cfg.WhatsAppDataDSN(),
		})
		if err != nil {
			slog.Warn("whatsapp channel: open data db", "account", acct.Label, "error", err)
			continue
		}

		sender := &whatsAppSender{client: client, db: db}
		ch := wachannel.NewChannel(sender, acct.OwnerJID)

		interactor := channel.NewInteractor(ch)
		account := s.resolveAccount()
		bridge := tools.NewTokenBridge(s.google, account)

		sm := wachannel.NewSessionManager(s.cfg, ch, p, providerName, modelName, wachannel.SessionManagerDeps{
			Interactor:   interactor,
			ScopeWaiter:  s.scopeWaiter,
			TokenBridge:  bridge,
			GoogleAuth:   s.google,
			Account:      account,
			AccountLabel: acct.Label,
			OwnerJID:     acct.OwnerJID,
		})

		wachannel.RegisterEventHandler(client.WM(), ch, acct.OwnerJID)
		go sm.Run(ctx)

		slog.Info("whatsapp channel started", "account", acct.Label, "owner_jid", acct.OwnerJID)
	}
}

// whatsAppSender implements wachannel.messageSender using the real whatsmeow client.
type whatsAppSender struct {
	client *wasrc.Client
	db     *store.DB
}

func (s *whatsAppSender) SendText(ctx context.Context, jid, text string) error {
	_, err := wasrc.SendText(ctx, s.client, s.db, wasrc.SendInput{
		ChatJID: jid,
		Text:    text,
	})
	return err
}

// migrateDBs runs database migrations once at startup for all configured sources.
func (s *Server) migrateDBs() {
	migrations := []struct {
		name   string
		driver string
		dsn    string
		fn     func(*store.DB) error
	}{
		{"applenotes", s.cfg.AppleNotes.Storage.Driver, s.cfg.AppleNotesDataDSN(), ansrc.Migrate},
		{"imessage", s.cfg.IMessage.Storage.Driver, s.cfg.IMessageDataDSN(), imsrc.Migrate},
		{"whatsapp", s.cfg.WhatsApp.Storage.Driver, s.cfg.WhatsAppDataDSN(), wasrc.Migrate},
		{"gmail", s.cfg.Gmail.Storage.Driver, s.cfg.GmailDataDSN(), gmailsrc.Migrate},
		{"contacts", s.cfg.Contacts.Storage.Driver, s.cfg.ContactsDataDSN(), contactsrc.Migrate},
		{"scheduler", s.cfg.Scheduler.Storage.Driver, s.cfg.SchedulerDataDSN(), schedsrc.Migrate},
	}
	for _, m := range migrations {
		db, err := store.Open(store.Config{Driver: m.driver, DSN: m.dsn})
		if err != nil {
			slog.Warn("migrate: open failed", "source", m.name, "error", err)
			continue
		}
		if err := m.fn(db); err != nil {
			slog.Warn("migrate: failed", "source", m.name, "error", err)
		}
		db.Close()
	}
}
