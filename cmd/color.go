package main

import (
	"io"

	"github.com/fatih/color"
)

type colorWriter struct {
	io.Writer
	*color.Color
}

func (c *colorWriter) Write(p []byte) (n int, err error) {
	c.Color.Fprint(c.Writer, string(p))
	return len(p), nil
}
