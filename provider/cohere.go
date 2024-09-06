package provider

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"github.com/daulet/cmd/config"

	co "github.com/cohere-ai/cohere-go/v2"
	cocli "github.com/cohere-ai/cohere-go/v2/client"
	core "github.com/cohere-ai/cohere-go/v2/core"
)

const COHERE_API_KEY = "COHERE_API_KEY"

func NewCohereProvider() (Provider, error) {
	apiKey := os.Getenv(COHERE_API_KEY)
	if apiKey == "" {
		return nil, fmt.Errorf("Set %s env variable to your Cohere API key. Get one at https://dashboard.cohere.com/api-keys", COHERE_API_KEY)
	}
	return &cohereProvider{client: cocli.NewClient(cocli.WithToken(apiKey))}, nil
}

var _ Provider = (*cohereProvider)(nil)

type cohereProvider struct {
	client *cocli.Client
}

func (p *cohereProvider) Stream(ctx context.Context, cfg *config.Config, msgs []*Message) (io.Reader, error) {
	var messages []*co.ChatMessage
	for _, msg := range msgs {
		switch msg.Role {
		case User:
			messages = append(messages, &co.ChatMessage{
				Role:    co.ChatMessageRoleUser,
				Message: msg.Content,
			})
		case Assistant:
			messages = append(messages, &co.ChatMessage{
				Role:    co.ChatMessageRoleChatbot,
				Message: msg.Content,
			})
		default:
			log.Fatalf("unknown role: %s", msg.Role)
		}
	}
	var model *string
	if cfg.Model[config.ModelTypeChat] != "" {
		model = config.Ref(cfg.Model[config.ModelTypeChat])
	}
	req := &co.ChatStreamRequest{
		ChatHistory: messages[:len(messages)-1],
		Message:     messages[len(messages)-1].Message,

		Model:            model,
		Temperature:      cfg.Temperature,
		P:                cfg.TopP,
		K:                cfg.TopK,
		FrequencyPenalty: cfg.FrequencyPenalty,
		PresencePenalty:  cfg.PresencePenalty,
	}
	for _, connector := range cfg.Connectors {
		req.Connectors = append(req.Connectors, &co.ChatConnector{Id: connector})
	}
	stream, err := p.client.ChatStream(ctx, req)
	if err != nil {
		return nil, err
	}
	// TODO stream.Close()
	return &cohereStreamReader{stream: stream}, nil
}

func (p *cohereProvider) Transcribe(ctx context.Context, cfg *config.Config, audio *AudioFile) ([]*AudioSegment, error) {
	return nil, fmt.Errorf("transcription is not supported by Cohere")
}

func (p *cohereProvider) ListModels(ctx context.Context) ([]string, error) {
	resp, err := p.client.Models.List(ctx, &co.ModelsListRequest{
		Endpoint: (*co.CompatibleEndpoint)(co.String(string(co.CompatibleEndpointChat))),
	})
	if err != nil {
		return nil, err
	}
	var modelNames []string
	for _, model := range resp.Models {
		modelNames = append(modelNames, *model.Name)
	}
	return modelNames, nil
}

func (p *cohereProvider) ListConnectors(ctx context.Context) ([]string, error) {
	resp, err := p.client.Connectors.List(ctx, &co.ConnectorsListRequest{})
	if err != nil {
		return nil, err
	}
	var connectorNames []string
	for _, connector := range resp.Connectors {
		connectorNames = append(connectorNames, connector.Id)
	}
	return connectorNames, nil
}

type cohereStreamReader struct {
	stream *core.Stream[co.StreamedChatResponse]
	buf    []byte
}

var _ io.Reader = (*cohereStreamReader)(nil)

func (r *cohereStreamReader) Read(p []byte) (int, error) {
	if len(r.buf) > 0 {
		n := copy(p, r.buf)
		if n < len(r.buf) {
			r.buf = r.buf[n:]
			return n, nil
		}
		r.buf = nil
		return n, nil
	}
	resp, err := r.stream.Recv()
	if err != nil {
		return 0, err
	}
	if resp.TextGeneration == nil {
		return 0, nil
	}
	out := []byte(resp.TextGeneration.Text)
	n := copy(p, out)
	if n < len(out) {
		r.buf = out[n:]
	}
	return n, nil
}
