package provider

import (
	"context"
	"io"
	"log"

	"github.com/daulet/cmd/config"
	"github.com/sashabaranov/go-openai"
)

const (
	DEFAULT_AUDIO_MODEL = "whisper-large-v3"
	DEFAULT_CHAT_MODEL  = "llama-3.1-8b-instant"
)

// Groq implements OpenAI API compatability.
func NewGroqProvider(apiKey string) Provider {
	config := openai.DefaultConfig(apiKey)
	config.BaseURL = "https://api.groq.com/openai/v1"
	client := openai.NewClientWithConfig(config)

	return &openAIProvider{client: client}
}

var _ Provider = (*openAIProvider)(nil)

type openAIProvider struct {
	client *openai.Client
}

func (p *openAIProvider) Stream(ctx context.Context, cfg *config.Config, msgs []*Message) (io.Reader, error) {
	var messages []openai.ChatCompletionMessage
	for _, msg := range msgs {
		switch msg.Role {
		case User:
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleUser,
				Content: msg.Content,
			})
		case Assistant:
			messages = append(messages, openai.ChatCompletionMessage{
				Role:    openai.ChatMessageRoleAssistant,
				Content: msg.Content,
			})
		default:
			log.Fatalf("unknown role: %s", msg.Role)
		}
	}

	model := DEFAULT_CHAT_MODEL
	if cfg.Model != nil {
		model = *cfg.Model
	}
	stream, err := p.client.CreateChatCompletionStream(ctx, openai.ChatCompletionRequest{
		Model:    model,
		Messages: messages,
	})
	if err != nil {
		return nil, err
	}
	return &openaiStreamReader{stream: stream}, nil
}

func (p *openAIProvider) Transcribe(ctx context.Context, cfg *config.Config, filename string) (string, error) {
	model := DEFAULT_AUDIO_MODEL
	if cfg.Model != nil {
		model = *cfg.Model
	}
	res, err := p.client.CreateTranscription(ctx, openai.AudioRequest{
		Model:    model,
		FilePath: filename,
	})
	if err != nil {
		return "", err
	}
	return res.Text, nil
}

func (p *openAIProvider) ListModels(ctx context.Context) ([]string, error) {
	models, err := p.client.ListModels(ctx)
	if err != nil {
		return nil, err
	}
	var modelNames []string
	for _, model := range models.Models {
		modelNames = append(modelNames, model.ID)
	}
	return modelNames, nil
}

func (p *openAIProvider) ListConnectors(ctx context.Context) ([]string, error) {
	return nil, nil
}

var _ io.Reader = (*openaiStreamReader)(nil)

type openaiStreamReader struct {
	stream *openai.ChatCompletionStream
	buf    []byte
}

func (r *openaiStreamReader) Read(p []byte) (int, error) {
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
	out := []byte(resp.Choices[0].Delta.Content)
	n := copy(p, out)
	if n < len(out) {
		r.buf = out[n:]
	}
	return n, nil
}
