package parser

import (
	"bytes"
	"io"
)

type Buffer struct {
	b *bytes.Buffer
}

var _ io.Writer = (*Buffer)(nil)

func NewBuffer() *Buffer {
	return &Buffer{b: &bytes.Buffer{}}
}

func (c *Buffer) Write(p []byte) (n int, err error) {
	c.b.Write(p)
	return len(p), nil
}

func (c *Buffer) String() string {
	return string(c.b.Bytes())
}
