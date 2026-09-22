//go:build integration

package testinfra

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

type Mailpit struct {
	SMTPAddr string
	apiURL   string
}

func StartMailpit(t *testing.T) Mailpit {
	t.Helper()
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "axllent/mailpit:latest",
			ExposedPorts: []string{"1025/tcp", "8025/tcp"},
			WaitingFor:   wait.ForHTTP("/api/v1/messages").WithPort("8025/tcp"),
		},
		Started: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	host, err := container.Host(ctx)
	require.NoError(t, err)
	smtp, err := container.MappedPort(ctx, "1025/tcp")
	require.NoError(t, err)
	api, err := container.MappedPort(ctx, "8025/tcp")
	require.NoError(t, err)
	return Mailpit{SMTPAddr: host + ":" + smtp.Port(), apiURL: "http://" + host + ":" + api.Port()}
}

type Email struct {
	Subject string
	To      []string
}

func (m Mailpit) Messages(t *testing.T) []Email {
	t.Helper()
	res, err := http.Get(m.apiURL + "/api/v1/messages")
	require.NoError(t, err)
	defer func() { _ = res.Body.Close() }()
	var body struct {
		Messages []struct {
			Subject string `json:"Subject"`
			To      []struct {
				Address string `json:"Address"`
			} `json:"To"`
		} `json:"messages"`
	}
	require.NoError(t, json.NewDecoder(res.Body).Decode(&body))
	var out []Email
	for _, msg := range body.Messages {
		e := Email{Subject: msg.Subject}
		for _, to := range msg.To {
			e.To = append(e.To, to.Address)
		}
		out = append(out, e)
	}
	return out
}
