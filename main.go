package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/daulet/llm-cli/cohere"
	"github.com/daulet/llm-cli/parser"

	co "github.com/cohere-ai/cohere-go/v2"
	cocli "github.com/cohere-ai/cohere-go/v2/client"
)

const apiKeyEnvVar = "COHERE_API_KEY"

var (
	client *cocli.Client

	chat = flag.Bool("chat", false, "Chat with the AI")
)

func runChat(ctx context.Context, in io.Reader, out io.Writer) error {
	var (
		r    = bufio.NewScanner(in)
		w    = NewFlushingWriter(bufio.NewWriter(out))
		msgs []*co.ChatMessage
	)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		w.WriteString("User> ")
		if !r.Scan() {
			return r.Err()
		}
		userMsg := r.Text()

		response, err := runMessage(ctx, msgs, userMsg, w)
		if err != nil {
			return err
		}

		msgs = append(msgs,
			&co.ChatMessage{
				Role:    co.ChatMessageRoleUser,
				Message: userMsg,
			},
			&co.ChatMessage{
				Role:    co.ChatMessageRoleChatbot,
				Message: response,
			},
		)
	}
}

func runMessage(
	ctx context.Context,
	msgs []*co.ChatMessage,
	msg string,
	out io.Writer,
) (string, error) {
	w := NewFlushingWriter(bufio.NewWriter(out))

	stream, err := client.ChatStream(ctx, &co.ChatStreamRequest{
		ChatHistory: msgs,
		Message:     msg,
	})
	if err != nil {
		return "", err
	}

	rr := io.TeeReader(cohere.ReadFrom(stream), w)
	p := &parser.Buffer{}
	_, err = io.Copy(p, rr)
	if err != nil {
		return "", err
	}
	stream.Close()
	w.WriteString("\n")

	blocks := p.CodeBlocks()
	if len(blocks) > 0 {
		w.WriteString("Code blocks detected:\n")
		for _, block := range blocks {
			if block.Lang == parser.HTML {
				path := fmt.Sprintf("%sindex.html", os.TempDir())
				if err := os.WriteFile(path, []byte(block.Code), 0644); err != nil {
					return "", err
				}
				if err := runCmd("open", fmt.Sprintf("file://%s", path)); err != nil {
					return "", err
				}
			}
		}
	}
	return p.String(), nil
}

func runCmd(prog string, args ...string) error {
	cmd := exec.Command(prog, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func main() {
	flag.Parse()
	ctx := context.Background()
	client = cocli.NewClient(cocli.WithToken(os.Getenv(apiKeyEnvVar)))

	var err error
	switch {
	case *chat:
		err = runChat(ctx, os.Stdin, os.Stdout)
	default:
		_, err = runMessage(ctx, nil /* chat history */, strings.Join(os.Args[1:], " "), os.Stdout)
	}
	if err != nil {
			panic(err)
	}
}
