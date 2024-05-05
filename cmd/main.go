package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/daulet/llm-cli/cohere"
	"github.com/daulet/llm-cli/config"
	"github.com/daulet/llm-cli/parser"

	co "github.com/cohere-ai/cohere-go/v2"
	cocli "github.com/cohere-ai/cohere-go/v2/client"
)

const apiKeyEnvVar = "COHERE_API_KEY"

var (
	client *cocli.Client
	cfg    *config.Config

	// config flags
	showConfig     = flag.Bool("config", false, "Show current config.")
	listModels     = flag.Bool("list-models", false, "List available models.")
	setModel       = flag.String("model", "", "Set model to use.")
	listConnectors = flag.Bool("list-connectors", false, "List available connectors.")
	setConnectors  = flag.String("connectors", "", "Set connectors to use.")
	setTemp        = flag.Float64("temperature", 0.0, "Set temperature value.")
	setTopP        = flag.Float64("top-p", 0.0, "Set top-p value.")
	setTopK        = flag.Int("top-k", 0, "Set top-k value.")
	setFreqPen     = flag.Float64("frequency-penalty", 0.0, "Set frequency penalty value.")
	setPresPen     = flag.Float64("presence-penalty", 0.0, "Set presence penalty value.")

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
	req := &co.ChatStreamRequest{
		ChatHistory: msgs[:len(msgs)-1],
		Message:     msgs[len(msgs)-1].Message,

		Model:            cfg.Model,
		Temperature:      cfg.Temperature,
		P:                cfg.TopP,
		K:                cfg.TopK,
		FrequencyPenalty: cfg.FrequencyPenalty,
		PresencePenalty:  cfg.PresencePenalty,
	}
	for _, connector := range cfg.Connectors {
		req.Connectors = append(req.Connectors, &co.ChatConnector{Id: connector})
	}
	stream, err := client.ChatStream(ctx, req)
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

func parseConfig(ctx context.Context) (bool, error) {
	var err error
	cfg, err = config.ReadConfig()
	if err != nil {
		return false, err
	}

	if *showConfig {
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return false, err
		}
		fmt.Println("Current config:")
		fmt.Println(string(data))
		return true, nil
	}

	if *listModels {
		resp, err := client.Models.List(ctx, &co.ModelsListRequest{
			Endpoint: (*co.CompatibleEndpoint)(co.String(string(co.CompatibleEndpointChat))),
		})
		if err != nil {
			return false, err
		}
		fmt.Println("Available models:")
		for _, model := range resp.Models {
			fmt.Println(*model.Name)
		}
		fmt.Println()
		fmt.Printf("Currently selected model: %s\n", *cfg.Model)
		return true, nil
	}

	if *listConnectors {
		resp, err := client.Connectors.List(ctx, &co.ConnectorsListRequest{})
		if err != nil {
			return false, err
		}
		fmt.Println("Available connectors:")
		for _, connector := range resp.Connectors {
			fmt.Println(connector.Id)
		}
		fmt.Println()
		fmt.Printf("Currently selected connectors: %s\n", cfg.Connectors)
		return true, nil
	}

	dirtyCfg := false
	flag.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "model":
			cfg.Model = setModel
			dirtyCfg = true
		case "connectors":
			cfg.Connectors = strings.Split(*setConnectors, ",")
			if *setConnectors == "" {
				cfg.Connectors = nil
			}
			dirtyCfg = true
		case "temperature":
			cfg.Temperature = setTemp
			dirtyCfg = true
		case "top-p":
			cfg.TopP = setTopP
			dirtyCfg = true
		case "top-k":
			cfg.TopK = setTopK
			dirtyCfg = true
		case "frequency-penalty":
			cfg.FrequencyPenalty = setFreqPen
			dirtyCfg = true
		case "presence-penalty":
			cfg.PresencePenalty = setPresPen
			dirtyCfg = true
		}
	})
	if dirtyCfg {
		if err := config.WriteConfig(cfg); err != nil {
			return false, err
		}
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return false, err
		}
		fmt.Println("Current config:")
		fmt.Println(string(data))
		return true, nil
	}
	return false, nil
}

func cmd(ctx context.Context) error {
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
	flag.Parse()

	client = cocli.NewClient(cocli.WithToken(os.Getenv(apiKeyEnvVar)))

	ctx := context.Background()
	done, err := parseConfig(ctx)
	if err != nil {
		panic(err)
	}
	if done {
		return
	}
	if err := cmd(ctx); err != nil {
		panic(err)
	}
}
