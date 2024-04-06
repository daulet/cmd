package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	co "github.com/cohere-ai/cohere-go/v2"
	cocli "github.com/cohere-ai/cohere-go/v2/client"
)

const apiKeyEnvVar = "COHERE_API_KEY"

func run(ctx context.Context) error {
	client := cocli.NewClient(cocli.WithToken(os.Getenv(apiKeyEnvVar)))
	scanner := bufio.NewScanner(os.Stdin)

	var msgs []*co.ChatMessage
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		fmt.Print("User> ")
		if !scanner.Scan() {
			return scanner.Err()
		}
		prompt := scanner.Text()
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
			fmt.Print(msg.TextGeneration.Text)
		}
		stream.Close()
		fmt.Println()
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
	if err := run(ctx); err != nil {
		panic(err)
	}
}
