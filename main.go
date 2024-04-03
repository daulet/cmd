package main

import (
	"context"
	"fmt"
	"os"

	co "github.com/cohere-ai/cohere-go/v2"
	cocli "github.com/cohere-ai/cohere-go/v2/client"
)

const apiKeyEnvVar = "COHERE_API_KEY"

func run() error {
	client := cocli.NewClient(cocli.WithToken(os.Getenv(apiKeyEnvVar)))
	response, err := client.Chat(
		context.TODO(),
		&co.ChatRequest{
			Message: "How is the weather today?",
		},
	)
	if err != nil {
		return err
	}
	fmt.Println(response.Text)
	return nil
}

func main() {
	if err := run(); err != nil {
		panic(err)
	}
}
