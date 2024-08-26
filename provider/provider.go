package provider

import (
	"context"
	"io"

	"github.com/daulet/cmd/config"
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

type AudioFile struct {
	FilePath string
	Reader   io.Reader
}

type AudioSegment struct {
	Text  string
	Seek  int
	Start float64
	End   float64
}

type Provider interface {
	ListModels(ctx context.Context) ([]string, error)
	ListConnectors(ctx context.Context) ([]string, error)
	Stream(ctx context.Context, cfg *config.Config, msgs []*Message) (io.Reader, error)
	Transcribe(ctx context.Context, cfg *config.Config, audio *AudioFile) ([]*AudioSegment, error)
}
