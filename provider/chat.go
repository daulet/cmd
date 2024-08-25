package provider

import (
	"context"
	"io"

	"github.com/daulet/llm-cli/config"
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
