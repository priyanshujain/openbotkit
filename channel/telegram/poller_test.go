package telegram

import (
	"strings"
	"sync"
	"testing"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type mockInterrupter struct {
	mu            sync.Mutex
	agentRunning  bool
	killCalled    bool
	delegateTasks []TaskSummary
	killedTaskID  string
}

func (m *mockInterrupter) IsAgentRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.agentRunning
}

func (m *mockInterrupter) Kill() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.agentRunning {
		return false
	}
	m.killCalled = true
	return true
}

func (m *mockInterrupter) RunningDelegateTasks() []TaskSummary {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.delegateTasks
}

func (m *mockInterrupter) KillDelegateTask(id string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, t := range m.delegateTasks {
		if t.ID == id {
			m.killedTaskID = id
			return true
		}
	}
	return false
}

func pollerSentTexts(bot *mockBot) []string {
	bot.mu.Lock()
	defer bot.mu.Unlock()
	var texts []string
	for _, c := range bot.sent {
		if msg, ok := c.(tgbotapi.MessageConfig); ok {
			texts = append(texts, msg.Text)
		}
	}
	return texts
}

func TestPoller_KillWhileAgentRunning(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{agentRunning: true}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	p.handleUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 123},
			Text: "/kill",
		},
	})

	inter.mu.Lock()
	defer inter.mu.Unlock()
	if !inter.killCalled {
		t.Fatal("expected Kill to be called")
	}
}

// racyInterrupter simulates the agent finishing between IsAgentRunning and Kill.
type racyInterrupter struct{ mockInterrupter }

func (r *racyInterrupter) IsAgentRunning() bool { return true }
func (r *racyInterrupter) Kill() bool           { return false }

func TestPoller_KillAgentFinishesBetweenCheckAndKill(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &racyInterrupter{}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	p.handleUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 123},
			Text: "/kill",
		},
	})

	texts := pollerSentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "Already finished") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Already finished' message, got: %v", texts)
	}
}

func TestPoller_KillNothingRunning(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{agentRunning: false}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	p.handleUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 123},
			Text: "/kill",
		},
	})

	texts := pollerSentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "Nothing running") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Nothing running' message, got: %v", texts)
	}
}

func TestPoller_KillWithDelegateTasks(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{
		agentRunning: false,
		delegateTasks: []TaskSummary{
			{ID: "abc123", Task: "research Go"},
		},
	}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	p.handleUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 123},
			Text: "/kill",
		},
	})

	texts := pollerSentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "background task") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected delegate task list, got: %v", texts)
	}
}

func TestPoller_MessageWhileAgentRunning(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{agentRunning: true}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	p.handleUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 10,
			From:      &tgbotapi.User{ID: 123},
			Text:      "hello",
		},
	})

	texts := pollerSentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "Want me to stop?") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected interrupt prompt, got: %v", texts)
	}

	p.mu.Lock()
	if !p.interruptPending {
		t.Error("expected interruptPending to be true")
	}
	if p.pendingMsg == nil || p.pendingMsg.text != "hello" {
		t.Error("expected pending message")
	}
	p.mu.Unlock()
}

func TestPoller_MessageWhileInterruptPending(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{agentRunning: true}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	// First message triggers interrupt
	p.handleUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 10,
			From:      &tgbotapi.User{ID: 123},
			Text:      "first",
		},
	})

	// Second message while interrupt pending — should queue
	p.handleUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 11,
			From:      &tgbotapi.User{ID: 123},
			Text:      "second",
		},
	})

	msg, _ := ch.ReceiveMessage()
	if msg.text != "second" {
		t.Errorf("expected queued 'second' message, got %q", msg.text)
	}
}

func TestPoller_InterruptStopCallback(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{agentRunning: true}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	// Trigger interrupt
	p.handleUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 10,
			From:      &tgbotapi.User{ID: 123},
			Text:      "hello",
		},
	})

	// Simulate stop callback
	p.handleInterruptCallback(callbackData{ID: "cb1", Data: "interrupt:stop"})

	inter.mu.Lock()
	if !inter.killCalled {
		t.Error("expected Kill to be called")
	}
	inter.mu.Unlock()

	p.mu.Lock()
	if p.interruptPending {
		t.Error("interrupt should be cleared")
	}
	if p.pendingMsg != nil {
		t.Error("pending message should be dropped")
	}
	p.mu.Unlock()
}

func TestPoller_InterruptContinueCallback(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{agentRunning: true}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	p.handleUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 10,
			From:      &tgbotapi.User{ID: 123},
			Text:      "queued msg",
		},
	})

	p.handleInterruptCallback(callbackData{ID: "cb1", Data: "interrupt:continue"})

	// The pending message should be queued for processing.
	msg, _ := ch.ReceiveMessage()
	if msg.text != "queued msg" {
		t.Errorf("expected 'queued msg', got %q", msg.text)
	}

	inter.mu.Lock()
	if inter.killCalled {
		t.Error("Kill should not have been called")
	}
	inter.mu.Unlock()
}

func TestPoller_InterruptCallbackAgentFinished(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{agentRunning: true}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	p.handleUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 10,
			From:      &tgbotapi.User{ID: 123},
			Text:      "hello",
		},
	})

	// Agent finishes before user responds
	inter.mu.Lock()
	inter.agentRunning = false
	inter.mu.Unlock()

	p.handleInterruptCallback(callbackData{ID: "cb1", Data: "interrupt:stop"})

	texts := pollerSentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "Already finished") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Already finished' message, got: %v", texts)
	}
}

func TestPoller_KillTaskCallback(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{
		delegateTasks: []TaskSummary{{ID: "abc12345", Task: "research"}},
	}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	p.handleKillTaskCallback(callbackData{ID: "cb1", Data: "kill_task:abc12345"})

	inter.mu.Lock()
	if inter.killedTaskID != "abc12345" {
		t.Errorf("expected killed task abc12345, got %q", inter.killedTaskID)
	}
	inter.mu.Unlock()

	texts := pollerSentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "Killed task") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Killed task' message, got: %v", texts)
	}
}

func TestPoller_KillTaskCallbackAll(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{
		delegateTasks: []TaskSummary{
			{ID: "t1", Task: "task one"},
			{ID: "t2", Task: "task two"},
		},
	}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	p.handleKillTaskCallback(callbackData{ID: "cb1", Data: "kill_task:all"})

	texts := pollerSentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "Killed 2 task(s)") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Killed 2 task(s)' message, got: %v", texts)
	}
}

func TestPoller_KillTaskCallbackUnknown(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	p.handleKillTaskCallback(callbackData{ID: "cb1", Data: "kill_task:nonexistent"})

	texts := pollerSentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "not found") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'not found' message, got: %v", texts)
	}
}

func TestPoller_KillNilInterrupter(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	p := &Poller{ownerID: 123, channel: ch, interrupter: nil}

	p.handleUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			From: &tgbotapi.User{ID: 123},
			Text: "/kill",
		},
	})

	texts := pollerSentTexts(bot)
	found := false
	for _, txt := range texts {
		if strings.Contains(txt, "Nothing running") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected 'Nothing running' message, got: %v", texts)
	}
}

func TestPoller_NormalMessageWhenIdle(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{agentRunning: false}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	p.handleUpdate(tgbotapi.Update{
		Message: &tgbotapi.Message{
			MessageID: 10,
			From:      &tgbotapi.User{ID: 123},
			Text:      "normal message",
		},
	})

	msg, _ := ch.ReceiveMessage()
	if msg.text != "normal message" {
		t.Errorf("expected 'normal message', got %q", msg.text)
	}
}

func TestPoller_ConcurrentKillAndHandle(t *testing.T) {
	bot := &mockBot{}
	ch := NewChannel(bot, 123)
	inter := &mockInterrupter{agentRunning: true}
	p := &Poller{ownerID: 123, channel: ch, interrupter: inter}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		p.handleUpdate(tgbotapi.Update{
			Message: &tgbotapi.Message{
				From: &tgbotapi.User{ID: 123},
				Text: "/kill",
			},
		})
	}()

	go func() {
		defer wg.Done()
		p.handleUpdate(tgbotapi.Update{
			Message: &tgbotapi.Message{
				MessageID: 10,
				From:      &tgbotapi.User{ID: 123},
				Text:      "concurrent msg",
			},
		})
	}()

	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for concurrent operations")
	}
}
