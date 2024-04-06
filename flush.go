package main

import "io"

type FlushingWriter struct {
	w     io.Writer
	flush func() error
}

func NewFlushingWriter(w io.Writer) *FlushingWriter {
	if f, ok := w.(interface{ Flush() error }); ok {
		return &FlushingWriter{
			w: w,
			flush: func() error {
				return f.Flush()
			},
		}
	}
	return &FlushingWriter{w: w}
}

func (w *FlushingWriter) Write(p []byte) (n int, err error) {
	n, err = w.w.Write(p)
	if err != nil {
		return n, err
	}
	if w.flush != nil {
		err = w.flush()
	}
	return n, err
}

func (w *FlushingWriter) WriteString(s string) (n int, err error) {
	return w.Write([]byte(s))
}
