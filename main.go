package main

import (
	"bufio"
	"context"
	"io"
	"os"

	"github.com/daulet/llm-cli/cohere"
	"github.com/daulet/llm-cli/parser"

	co "github.com/cohere-ai/cohere-go/v2"
	cocli "github.com/cohere-ai/cohere-go/v2/client"
)

const apiKeyEnvVar = "COHERE_API_KEY"

func write(out *bufio.Writer, s string) {
	if _, err := out.Write([]byte(s)); err != nil {
		panic(err)
	}
	out.Flush()
}

func run(ctx context.Context, in io.Reader, out io.Writer) error {
	var (
		r    = bufio.NewScanner(in)
		w    = bufio.NewWriter(out)
		cl   = cocli.NewClient(cocli.WithToken(os.Getenv(apiKeyEnvVar)))
		msgs []*co.ChatMessage
	)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		write(w, "User> ")
		if !r.Scan() {
			return r.Err()
		}
		userMsg := r.Text()
		stream, err := cl.ChatStream(ctx, &co.ChatStreamRequest{
			ChatHistory: msgs,
			Message:     userMsg,
		})
		if err != nil {
			return err
		}

		rr := io.TeeReader(cohere.ReadFrom(stream), w)
		var parser = parser.NewBuffer()
		_, err = io.Copy(parser, rr)
		if err != nil {
			return err
		}
		stream.Close()
		write(w, "\n")

		msgs = append(msgs,
			&co.ChatMessage{
				Role:    co.ChatMessageRoleUser,
				Message: userMsg,
			},
			&co.ChatMessage{
				Role:    co.ChatMessageRoleChatbot,
				Message: parser.String(),
			},
		)
	}
}

func main() {
	ctx := context.Background()
	if err := run(ctx, os.Stdin, os.Stdout); err != nil {
		panic(err)
	}
}
