package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

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
		response, err := client.Chat(
			ctx,
			&co.ChatRequest{
				ChatHistory: msgs,
				Message:     prompt,
			},
		)
		if err != nil {
			return err
		}
		msgs = append(msgs,
			&co.ChatMessage{
				Role:    co.ChatMessageRoleUser,
				Message: prompt,
			},
			&co.ChatMessage{
				Role:    co.ChatMessageRoleChatbot,
				Message: response.Text,
			},
		)
		fmt.Println(response.Text)
	}
}

func main() {
	ctx := context.Background()
	if err := run(ctx); err != nil {
		panic(err)
	}
}
