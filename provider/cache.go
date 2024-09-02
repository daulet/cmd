package provider

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/daulet/cmd/config"
)

type ProviderCloser interface {
	Provider
	io.Closer
}

func NewCacheProvider(p Provider, cachePath string) (ProviderCloser, error) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return &cacheProvider{p: p, c: &cache{AudioSegments: make(map[string][]*AudioSegment)}, cachePath: cachePath}, nil
		}
		return nil, err
	}
	cache := &cache{
		AudioSegments: make(map[string][]*AudioSegment),
	}
	if err := json.Unmarshal(data, cache); err != nil {
		return nil, err
	}
	return &cacheProvider{p: p, c: cache, cachePath: cachePath}, nil
}

var _ ProviderCloser = (*cacheProvider)(nil)

type cacheProvider struct {
	p         Provider
	c         *cache
	cachePath string
}

// ListConnectors implements Provider.
func (c *cacheProvider) ListConnectors(ctx context.Context) ([]string, error) {
	return c.p.ListConnectors(ctx)
}

// ListModels implements Provider.
func (c *cacheProvider) ListModels(ctx context.Context) ([]string, error) {
	return c.p.ListModels(ctx)
}

// Stream implements Provider.
func (c *cacheProvider) Stream(ctx context.Context, cfg *config.Config, msgs []*Message) (io.Reader, error) {
	return c.p.Stream(ctx, cfg, msgs)
}

// Transcribe implements Provider.
func (c *cacheProvider) Transcribe(ctx context.Context, cfg *config.Config, audio *AudioFile) ([]*AudioSegment, error) {
	data, err := io.ReadAll(audio.Reader)
	if err != nil {
		return nil, err
	}
	audio.Reader = bytes.NewReader(data)
	hash := sha256.Sum256(data)
	key := base64.URLEncoding.EncodeToString(hash[:])
	if c.c.AudioSegments[key] != nil {
		return c.c.AudioSegments[key], nil
	}
	res, err := c.p.Transcribe(ctx, cfg, audio)
	if err != nil {
		return nil, err
	}
	c.c.AudioSegments[key] = res
	return res, nil
}

func (c *cacheProvider) Close() error {
	if err := os.MkdirAll(filepath.Dir(c.cachePath), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c.c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.cachePath, data, 0644)
}

type cache struct {
	AudioSegments map[string][]*AudioSegment `json:"audio_segments,omitempty"`
}
