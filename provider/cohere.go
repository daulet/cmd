package provider

import (
	"context"
	"io"
	"log"

	"github.com/daulet/llm-cli/config"

	co "github.com/cohere-ai/cohere-go/v2"
	cocli "github.com/cohere-ai/cohere-go/v2/client"
	core "github.com/cohere-ai/cohere-go/v2/core"
)

func NewCohereProvider(apiKey string) *CohereProvider {
	return &CohereProvider{client: cocli.NewClient(cocli.WithToken(apiKey))}
}

var _ Provider = (*CohereProvider)(nil)

type CohereProvider struct {
	client *cocli.Client
}

func (p *CohereProvider) Stream(ctx context.Context, cfg *config.Config, msgs []*Message) (io.Reader, error) {
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
	req := &co.ChatStreamRequest{
		ChatHistory: messages[:len(messages)-1],
		Message:     messages[len(messages)-1].Message,

		Model:            cfg.Model,
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

func (p *CohereProvider) ListModels(ctx context.Context) ([]string, error) {
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

func (p *CohereProvider) ListConnectors(ctx context.Context) ([]string, error) {
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
