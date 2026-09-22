package mail

import (
	"context"
	"fmt"
	"net/smtp"
	"strings"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
)

type SMTPNotifier struct {
	addr string
	from string
}

func NewSMTPNotifier(addr, from string) *SMTPNotifier {
	return &SMTPNotifier{addr: addr, from: from}
}

func (n *SMTPNotifier) Send(_ context.Context, notification video.Notification) error {
	msg := Compose(notification)
	var b strings.Builder
	fmt.Fprintf(&b, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s", n.from, msg.To, msg.Subject, msg.Text)
	if err := smtp.SendMail(n.addr, nil, n.from, []string{msg.To}, []byte(b.String())); err != nil {
		return fmt.Errorf("smtp: send: %w", err)
	}
	return nil
}
