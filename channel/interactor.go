package channel

import "github.com/73ai/openbotkit/agent/tools"

// Interactor adapts a Channel to the tools.Interactor interface.
// It exposes Notify, NotifyLink, and RequestApproval but not Receive.
type Interactor struct {
	ch Channel
}

// Compile-time check.
var _ tools.Interactor = (*Interactor)(nil)

func NewInteractor(ch Channel) *Interactor {
	return &Interactor{ch: ch}
}

func (i *Interactor) Notify(msg string) error                   { return i.ch.Send(msg) }
func (i *Interactor) NotifyLink(text, url string) error         { return i.ch.SendLink(text, url) }
func (i *Interactor) RequestApproval(desc string) (bool, error) { return i.ch.RequestApproval(desc) }
