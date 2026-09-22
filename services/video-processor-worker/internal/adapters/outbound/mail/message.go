package mail

import (
	"fmt"

	"github.com/ivanzao/fiap-video-processor/services/video-processor-worker/internal/domain/video"
)

type Message struct {
	To      string
	Subject string
	Text    string
}

func Compose(n video.Notification) Message {
	if n.Outcome == video.OutcomeCompleted {
		return Message{
			To:      n.To,
			Subject: fmt.Sprintf("Seu vídeo %s foi processado", n.Filename),
			Text: fmt.Sprintf("Olá!\n\nO processamento do vídeo %s terminou com sucesso: %d frames foram extraídos.\n"+
				"Acesse a plataforma para baixar o arquivo zip.\n\nPedido: %s\n", n.Filename, n.FrameCount, n.RequestID),
		}
	}
	return Message{
		To:      n.To,
		Subject: fmt.Sprintf("Falha ao processar o vídeo %s", n.Filename),
		Text: fmt.Sprintf("Olá!\n\nNão foi possível processar o vídeo %s.\nMotivo: %s\n\n"+
			"Você pode enviar o vídeo novamente pela plataforma.\n\nPedido: %s\n", n.Filename, n.Reason, n.RequestID),
	}
}
