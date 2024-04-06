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
	HTML
	Python
)

func language(s string) Language {
	switch s {
	case "go":
		return Go
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

type Buffer struct {
	b []byte
}

var _ io.Writer = (*Buffer)(nil)

func (c *Buffer) Write(p []byte) (n int, err error) {
	c.b = append(c.b, p...)
	return len(p), nil
}

func (c *Buffer) CodeBlocks() []*CodeBlock {
	var blocks []*CodeBlock
	buf := bufio.NewScanner(bytes.NewReader(c.b))
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
			blocks = append(blocks, &CodeBlock{
				Lang: lang,
				Code: block.String(),
			})
		}
	}
	return blocks
}

func (c *Buffer) String() string {
	return string(c.b)
}
