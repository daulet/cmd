package main

import (
	"bufio"
	"context"
	"io"
	"os"
	"strings"

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
		reader = bufio.NewScanner(in)
		writer = bufio.NewWriter(out)
		client = cocli.NewClient(cocli.WithToken(os.Getenv(apiKeyEnvVar)))
		msgs   []*co.ChatMessage
	)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		write(writer, "User> ")
		if !reader.Scan() {
			return reader.Err()
		}
		prompt := reader.Text()
		stream, err := client.ChatStream(ctx, &co.ChatStreamRequest{
			ChatHistory: msgs,
			Message:     prompt,
		})
		if err != nil {
			return err
		}
		var response strings.Builder
		for msg, err := stream.Recv(); err != io.EOF; msg, err = stream.Recv() {
			if err != nil {
				return err
			}
			if msg.TextGeneration == nil {
				continue
			}
			response.WriteString(msg.TextGeneration.Text)
			write(writer, msg.TextGeneration.Text)
		}
		stream.Close()
		write(writer, "\n")

		msgs = append(msgs,
			&co.ChatMessage{
				Role:    co.ChatMessageRoleUser,
				Message: prompt,
			},
			&co.ChatMessage{
				Role:    co.ChatMessageRoleChatbot,
				Message: response.String(),
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
