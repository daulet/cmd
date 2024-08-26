package provider

import (
	"context"
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
			return &cacheProvider{p: p, c: &cache{Data: map[string]string{}}, cachePath: cachePath}, nil
		}
		return nil, err
	}
	cache := &cache{}
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
// TODO change filename to contents to improve cache hit rate
func (c *cacheProvider) Transcribe(ctx context.Context, cfg *config.Config, filename string) (string, error) {
	if c.c.Data[filename] != "" {
		return c.c.Data[filename], nil
	}
	res, err := c.p.Transcribe(ctx, cfg, filename)
	if err != nil {
		return "", err
	}
	c.c.Data[filename] = res
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
	Data map[string]string
}
