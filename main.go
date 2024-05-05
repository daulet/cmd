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

	chat = flag.Bool("chat", false, "Chat with the AI")
	// TODO have separate flags for code and command execution
	// with command you want right away
	// with code (since it is longer) you want to see progress and execute later
	execute = flag.Bool("exec", false, "Execute generated command/code")
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

	var codeW io.WriteCloser
	if *execute {
		var blockCh <-chan *parser.CodeBlock
		codeW, blockCh = parser.NewCode()
		go func() {
			for block := range blockCh {
				switch block.Lang {
				case parser.Bash:
					if err := runCmd("bash", "-c", block.Code); err != nil {
						fmt.Println(err) // TODO
					}
				case parser.HTML:
					path := fmt.Sprintf("%sindex.html", os.TempDir())
					if err := os.WriteFile(path, []byte(block.Code), 0644); err != nil {
						fmt.Println(err) // TODO
					}
					if err := runCmd("open", fmt.Sprintf("file://%s", path)); err != nil {
						fmt.Println(err) // TODO
					}
				}
			}
		}()
	}

	buf := bytes.NewBuffer(nil)
	// tee the stream to buffer to catch full output
	river := io.TeeReader(cohere.ReadFrom(stream), buf)
	if codeW != nil {
		// tee the stream to code parser
		river = io.TeeReader(river, codeW)
	}
	// write to actual output
	_, err = io.Copy(w, river)
	if err != nil {
		return "", err
	}
	if codeW != nil {
		codeW.Close()
	}
	stream.Close()
	w.WriteString("\n")
	return buf.String(), nil
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
		var out io.Writer = os.Stdout
		if *execute {
			out = io.Discard
		}
		_, err = runMessage(ctx, nil /* chat history */, strings.Join(flag.Args(), " "), out)
	}
	if err != nil {
		panic(err)
	}
}
