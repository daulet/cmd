package provider

import (
	"io"

	co "github.com/cohere-ai/cohere-go/v2"
	core "github.com/cohere-ai/cohere-go/v2/core"
)

type streamReader struct {
	stream *core.Stream[co.StreamedChatResponse]
	buf    []byte
}

var _ io.Reader = (*streamReader)(nil)

func (r *streamReader) Read(p []byte) (int, error) {
	if len(r.buf) > 0 {
		n := copy(p, r.buf)
		if n < len(r.buf) {
			r.buf = r.buf[n:]
			return n, nil
		}
		r.buf = nil
		return n, nil
	}
	resp, err := r.stream.Recv()
	if err != nil {
		return 0, err
	}
	if resp.TextGeneration == nil {
		return 0, nil
	}
	out := []byte(resp.TextGeneration.Text)
	n := copy(p, out)
	if n < len(out) {
		r.buf = out[n:]
	}
	return n, nil
}

func ReadFrom(stream *core.Stream[co.StreamedChatResponse]) io.Reader {
	return &streamReader{
		stream: stream,
	}
}
