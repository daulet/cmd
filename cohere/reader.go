package cohere

import (
	"io"

	co "github.com/cohere-ai/cohere-go/v2"
	core "github.com/cohere-ai/cohere-go/v2/core"
)

type streamReader struct {
	stream *core.Stream[co.StreamedChatResponse]
}

var _ io.Reader = (*streamReader)(nil)

func (r *streamReader) Read(p []byte) (n int, err error) {
	resp, err := r.stream.Recv()
	if err != nil {
		return 0, err
	}
	if resp.TextGeneration == nil {
		return 0, nil
	}
	n = copy(p, []byte(resp.TextGeneration.Text))
	return n, nil
}

func ReadFrom(stream *core.Stream[co.StreamedChatResponse]) io.Reader {
	return &streamReader{stream}
}
