package channel

import (
	"testing"

	"github.com/73ai/openbotkit/agent/tools"
)

type mockChannel struct {
	sent    []string
	links   []struct{ text, url string }
	approve bool
}

func (m *mockChannel) Send(msg string) error {
	m.sent = append(m.sent, msg)
	return nil
}
func (m *mockChannel) Receive() (string, error) { return "", nil }
func (m *mockChannel) RequestApproval(action string) (bool, error) {
	return m.approve, nil
}
func (m *mockChannel) SendLink(text, url string) error {
	m.links = append(m.links, struct{ text, url string }{text, url})
	return nil
}

// Compile-time check: Interactor implements tools.Interactor.
var _ tools.Interactor = (*Interactor)(nil)

func TestInteractor_DelegatesNotify(t *testing.T) {
	ch := &mockChannel{}
	inter := NewInteractor(ch)
	if err := inter.Notify("hello"); err != nil {
		t.Fatal(err)
	}
	if len(ch.sent) != 1 || ch.sent[0] != "hello" {
		t.Errorf("sent = %v", ch.sent)
	}
}

func TestInteractor_DelegatesNotifyLink(t *testing.T) {
	ch := &mockChannel{}
	inter := NewInteractor(ch)
	if err := inter.NotifyLink("click", "https://x.com"); err != nil {
		t.Fatal(err)
	}
	if len(ch.links) != 1 || ch.links[0].url != "https://x.com" {
		t.Errorf("links = %v", ch.links)
	}
}

func TestInteractor_DelegatesApproval(t *testing.T) {
	ch := &mockChannel{approve: true}
	inter := NewInteractor(ch)
	ok, err := inter.RequestApproval("delete stuff")
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Error("expected approval")
	}
}
