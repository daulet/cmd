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
	case "python":
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
	// TODO maybe split with a TeeReader
	// duplicate buffer to get all bytes in String()
	b      []byte
	data   chan []byte
	blocks chan *CodeBlock
}

var _ io.Writer = (*Code)(nil)
var _ io.Closer = (*Code)(nil)

func (c *Code) Write(p []byte) (n int, err error) {
	c.b = append(c.b, p...)
	c.data <- p
	return len(p), nil
}

func (c *Code) Close() error {
	close(c.data)
	return nil
}

// Read to implement io.Reader so we can use bufio.Scanner
func (c *Code) Read(p []byte) (n int, err error) {
	data, ok := <-c.data
	if !ok {
		return 0, io.EOF
	}
	// TODO what if len(p) < len(data)
	return copy(p, data), nil
}

func (c *Code) CodeBlocks() <-chan *CodeBlock {
	return c.blocks
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

func (c *Code) String() string {
	return string(c.b)
}

// TODO without someone reading from here the whole reading/writing will block
func NewCode() *Code {
	buf := &Code{
		data:   make(chan []byte),
		blocks: make(chan *CodeBlock),
	}
	go func() {
		defer close(buf.blocks)
		scanBlocks(buf, buf.blocks)
	}()
	return buf
}
