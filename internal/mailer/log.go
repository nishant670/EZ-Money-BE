package mailer

import (
	"context"
	"log"
)

// logSender prints the message instead of sending it. It exists so that local
// development has a working OTP flow without a mail account, and it is the one
// driver that must never be selected in production — it would print live codes
// into the platform log.
type logSender struct {
	from Address
}

func NewLogSender(from Address) Sender { return &logSender{from: from} }

func (l *logSender) Name() string { return "log" }

func (l *logSender) Send(_ context.Context, msg Message) error {
	if err := msg.Validate(); err != nil {
		return err
	}
	log.Printf("[mailer:log] to=%s subject=%q\n%s", msg.To, msg.Subject, msg.Text)
	return nil
}
