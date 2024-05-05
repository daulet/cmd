package main

import (
	"bufio"
	"bytes"
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

	// config flags
	listModels = flag.Bool("list-models", false, "List available models.")

	chat    = flag.Bool("chat", false, "Start chat session with LLM, other flags apply.")
	execute = flag.Bool("exec", false, "Execute generated command/code, do not show LLM output.")
	run     = flag.Bool("run", false, "Stream LLM output and run generated command/code at the end.")
)

func multiTurn(
	ctx context.Context,
	out io.WriteCloser,
	in io.Reader,
	turnFn func(context.Context, io.WriteCloser, []*co.ChatMessage) (string, error),
) error {
	var (
		r    = bufio.NewScanner(in)
		msgs []*co.ChatMessage
	)
	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		out.Write([]byte("User> ")) // TODO does this polute the output?
		if !r.Scan() {
			return r.Err()
		}
		userMsg := r.Text()

		msgs = append(msgs,
			&co.ChatMessage{
				Role:    co.ChatMessageRoleUser,
				Message: userMsg,
			})

		botMsg, err := turnFn(ctx, out, msgs)
		if err != nil {
			return err
		}

		msgs = append(msgs,
			&co.ChatMessage{
				Role:    co.ChatMessageRoleChatbot,
				Message: botMsg,
			},
		)
	}
}

func generate(
	ctx context.Context,
	out io.WriteCloser,
	msgs []*co.ChatMessage,
) (string, error) {
	buf := bytes.NewBuffer(nil)
	stream, err := client.ChatStream(ctx, &co.ChatStreamRequest{
		ChatHistory: msgs[:len(msgs)-1],
		Message:     msgs[len(msgs)-1].Message,
	})
	if err != nil {
		return "", err
	}
	_, err = io.Copy(parser.MultiWriter(out, buf), cohere.ReadFrom(stream))
	if err != nil {
		return "", err
	}
	stream.Close()
	out.Write([]byte("\n"))
	return buf.String(), nil
}

func runBlock(block *parser.CodeBlock) error {
	switch block.Lang {
	case parser.Bash:
		if err := runCmd("bash", "-c", block.Code); err != nil {
			return err
		}
	case parser.HTML:
		path := fmt.Sprintf("%sindex.html", os.TempDir())
		if err := os.WriteFile(path, []byte(block.Code), 0644); err != nil {
			return err
		}
		if err := runCmd("open", fmt.Sprintf("file://%s", path)); err != nil {
			return err
		}
	}
	return nil
}

func runCmd(prog string, args ...string) error {
	cmd := exec.Command(prog, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func cmd() error {
	ctx := context.Background()

	flag.Parse()
	client = cocli.NewClient(cocli.WithToken(os.Getenv(apiKeyEnvVar)))

	if *listModels {
		resp, err := client.Models.List(ctx, &co.ModelsListRequest{
			Endpoint: (*co.CompatibleEndpoint)(co.String(string(co.CompatibleEndpointChat))),
		})
		if err != nil {
			return err
		}
		fmt.Println("Available models:")
		for _, model := range resp.Models {
			fmt.Println(*model.Name)
		}
		return nil
	}

	turnFn := func(ctx context.Context, out io.WriteCloser, msgs []*co.ChatMessage) (string, error) {
		var blocks []*parser.CodeBlock
		done := make(chan struct{})

		switch {
		case *execute:
			codeW, blockCh := parser.NewCode()
			go func() {
				defer close(done)
				for block := range blockCh {
					runBlock(block)
				}
			}()
			// no output to the user, we just execute the code
			out = codeW
		case *run:
			codeW, blockCh := parser.NewCode()
			go func() {
				defer close(done)
				for block := range blockCh {
					blocks = append(blocks, block)
				}
			}()
			// we output generation to the user, then execute the code
			out = parser.MultiWriter(codeW, out)
		default:
			close(done)
		}

		response, err := generate(ctx, out, msgs)
		if err != nil {
			return "", err
		}

		out.Close()
		<-done
		for _, block := range blocks {
			runBlock(block)
		}

		return response, nil
	}

	var err error
	switch {
	case *chat:
		err = multiTurn(ctx, os.Stdout, os.Stdin, turnFn)
	default:
		_, err = turnFn(ctx, os.Stdout, []*co.ChatMessage{
			{
				Role:    co.ChatMessageRoleUser,
				Message: strings.Join(flag.Args(), " ")},
		})
	}
	if err != nil {
		return err
	}
	return nil
}

func main() {
	if err := cmd(); err != nil {
		panic(err)
	}
}
