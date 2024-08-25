package cohere

import (
	"context"
	"io"
	"log"

	"github.com/daulet/llm-cli/config"

	co "github.com/cohere-ai/cohere-go/v2"
	cocli "github.com/cohere-ai/cohere-go/v2/client"
)

type Role string

const (
	User      Role = "user"
	Assistant Role = "assistant"
)

type Message struct {
	Role    Role
	Content string
}

type Provider interface {
	Stream(ctx context.Context, cfg *config.Config, msgs []*Message) (io.Reader, error)
	ListModels(ctx context.Context) ([]string, error)
	ListConnectors(ctx context.Context) ([]string, error)
}

var _ Provider = (*CohereProvider)(nil)

func NewCohereProvider(apiKey string) *CohereProvider {
	return &CohereProvider{client: cocli.NewClient(cocli.WithToken(apiKey))}
}

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
	return ReadFrom(stream), nil
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
