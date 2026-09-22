package mail

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
)

type MailerSendNotifier struct {
	client   *http.Client
	endpoint string
	token    string
	from     string
}

func NewMailerSendNotifier(client *http.Client, endpoint, token, from string) *MailerSendNotifier {
	return &MailerSendNotifier{client: client, endpoint: endpoint, token: token, from: from}
}

type address struct {
	Email string `json:"email"`
}

type mailerSendRequest struct {
	From    address   `json:"from"`
	To      []address `json:"to"`
	Subject string    `json:"subject"`
	Text    string    `json:"text"`
}

func (n *MailerSendNotifier) Send(ctx context.Context, notification video.Notification) error {
	msg := Compose(notification)
	body, err := json.Marshal(mailerSendRequest{From: address{n.from}, To: []address{{msg.To}}, Subject: msg.Subject, Text: msg.Text})
	if err != nil {
		return fmt.Errorf("mailersend: encode: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.endpoint+"/v1/email", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("mailersend: request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+n.token)
	req.Header.Set("Content-Type", "application/json")
	res, err := n.client.Do(req)
	if err != nil {
		return fmt.Errorf("mailersend: send: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	if res.StatusCode >= 300 {
		detail, _ := io.ReadAll(io.LimitReader(res.Body, 4096))
		return fmt.Errorf("mailersend: status %d: %s", res.StatusCode, detail)
	}
	return nil
}
