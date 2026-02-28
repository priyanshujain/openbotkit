package whatsapp

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waCompanionReg"
	waStore "go.mau.fi/whatsmeow/store"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
)

func init() {
	waStore.DeviceProps.PlatformType = waCompanionReg.DeviceProps_DESKTOP.Enum()
	waStore.SetOSInfo("OpenBotKit", [3]uint32{0, 1, 0})
}

type Client struct {
	mu     sync.Mutex
	wm     *whatsmeow.Client
	dbPath string
	store  *sqlstore.Container
}

func NewClient(ctx context.Context, sessionDBPath string) (*Client, error) {
	container, err := sqlstore.New(ctx, "sqlite",
		fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)", sessionDBPath),
		waLog.Noop,
	)
	if err != nil {
		return nil, fmt.Errorf("open session store: %w", err)
	}

	device, err := container.GetFirstDevice(ctx)
	if err != nil {
		return nil, fmt.Errorf("get device: %w", err)
	}

	wm := whatsmeow.NewClient(device, waLog.Noop)
	return &Client{wm: wm, dbPath: sessionDBPath, store: container}, nil
}

func (c *Client) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.wm.Connect()
}

func (c *Client) ConnectWithQR(ctx context.Context, qrChan chan string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	ch, err := c.wm.GetQRChannel(ctx)
	if err != nil {
		return fmt.Errorf("get qr channel: %w", err)
	}

	if err := c.wm.Connect(); err != nil {
		return fmt.Errorf("connect: %w", err)
	}

	for evt := range ch {
		switch evt.Event {
		case "code":
			qrChan <- evt.Code
		case "success":
			close(qrChan)
			return nil
		case "timeout":
			close(qrChan)
			return fmt.Errorf("qr code scan timed out")
		}
	}

	close(qrChan)
	return nil
}

func (c *Client) Disconnect() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.wm.Disconnect()
}

func (c *Client) IsAuthenticated() bool {
	return c.wm.Store.ID != nil
}

func (c *Client) WM() *whatsmeow.Client {
	return c.wm
}

func (c *Client) ReconnectWithBackoff(ctx context.Context) error {
	delays := []time.Duration{2, 4, 8, 16, 30}

	for attempt := 0; ; attempt++ {
		idx := int(math.Min(float64(attempt), float64(len(delays)-1)))
		delay := delays[idx] * time.Second

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		c.mu.Lock()
		err := c.wm.Connect()
		c.mu.Unlock()

		if err == nil {
			return nil
		}
		fmt.Printf("reconnect attempt %d failed: %v\n", attempt+1, err)
	}
}
