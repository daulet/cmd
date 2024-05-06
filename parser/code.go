package parser

import (
	"bufio"
	"bytes"
	"io"
	"strings"
)

type Language int

const (
	Unknown Language = iota
	Go
	Bash
	HTML
	Python
)

func language(s string) Language {
	switch s {
	case "go":
		return Go
	case "bash":
		return Bash
	case "html":
		return HTML
	case "python", "python3":
		return Python
	default:
		return Unknown
	}
}

type CodeBlock struct {
	Lang Language
	Code string
}

type Code struct {
	data chan []byte
	buf  []byte
}

var _ io.WriteCloser = (*Code)(nil)

func (c *Code) Write(p []byte) (int, error) {
	// don't reuse p since it could be reused by the caller
	cp := make([]byte, len(p))
	copy(cp, p)
	c.data <- cp
	return len(p), nil
}

func (c *Code) Close() error {
	close(c.data)
	return nil
}

// Read to implement io.Reader so we can use bufio.Scanner
func (c *Code) Read(p []byte) (int, error) {
	if len(c.buf) > 0 {
		n := copy(p, c.buf)
		if n < len(c.buf) {
			c.buf = c.buf[n:]
			return n, nil
		}
		c.buf = nil
		return n, nil
	}
	data, ok := <-c.data
	if !ok {
		return 0, io.EOF
	}
	n := copy(p, data)
	if n < len(data) {
		c.buf = data[n:]
	}
	return n, nil
}

func scanBlocks(r io.Reader, blocks chan<- *CodeBlock) {
	buf := bufio.NewScanner(r)
	buf.Split(bufio.ScanLines)
	for buf.Scan() {
		line := buf.Text()
		if strings.HasPrefix(line, "```") {
			lang := language(strings.TrimPrefix(line, "```"))
			var block bytes.Buffer
			for buf.Scan() {
				line = buf.Text()
				if line == "```" {
					break
				}
				block.WriteString(line)
				block.WriteString("\n")
			}
			blocks <- &CodeBlock{
				Lang: lang,
				Code: block.String(),
			}
		}
	}
}

func NewCode() (*Code, <-chan *CodeBlock) {
	buf := &Code{data: make(chan []byte)}
	blocks := make(chan *CodeBlock)
	go func() {
		defer close(blocks)
		scanBlocks(buf, blocks)
	}()
	return buf, blocks
}
