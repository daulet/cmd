package parser

import (
	"io"
	"os"
)

// multiWriter exists because std doesn't implement Closer
type multiWriter struct {
	writers []io.Writer
}

var _ io.Writer = (*multiWriter)(nil)

func (t *multiWriter) Write(p []byte) (n int, err error) {
	for _, w := range t.writers {
		n, err = w.Write(p)
		if err != nil {
			return
		}
		if n != len(p) {
			err = io.ErrShortWrite
			return
		}
	}
	return len(p), nil
}

var _ io.Closer = (*multiWriter)(nil)

func (t *multiWriter) Close() error {
	var err error
	for _, w := range t.writers {
		if w == os.Stdout {
			// do not close stdout
			continue
		}
		if c, ok := w.(io.Closer); ok {
			if cerr := c.Close(); cerr != nil && err == nil {
				err = cerr
			}
		}
	}
	return err
}

// MultiWriter creates a writer that duplicates its writes to all the
// provided writers, similar to the Unix tee(1) command.
//
// Each write is written to each listed writer, one at a time.
// If a listed writer returns an error, that overall write operation
// stops and returns the error; it does not continue down the list.
func MultiWriter(writers ...io.Writer) io.WriteCloser {
	allWriters := make([]io.Writer, 0, len(writers))
	for _, w := range writers {
		if mw, ok := w.(*multiWriter); ok {
			allWriters = append(allWriters, mw.writers...)
		} else {
			allWriters = append(allWriters, w)
		}
	}
	return &multiWriter{allWriters}
}
