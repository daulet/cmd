package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/daulet/llm-cli/config"
	"github.com/daulet/llm-cli/parser"
	"github.com/daulet/llm-cli/provider"
)

const (
	COHERE_API_KEY = "COHERE_API_KEY"
	GROQ_API_KEY   = "GROQ_API_KEY"
)

var (
	prov provider.Provider
	cfg  *config.Config

	// config flags
	showConfig     = flag.Bool("config", false, "Show current config.")
	listModels     = flag.Bool("list-models", false, "List available models.")
	setModel       = flag.String("model", "", "Set model to use.")
	listConnectors = flag.Bool("list-connectors", false, "List available connectors.")
	setConnectors  = flag.String("connectors", "", "Set comma delimited list of connectors to use.")
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
	turnFn func(context.Context, io.WriteCloser, []*provider.Message) (string, error),
) error {
	var (
		r    = bufio.NewScanner(in)
		msgs []*provider.Message
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
			&provider.Message{
				Role:    provider.User,
				Content: userMsg,
			})

		botMsg, err := turnFn(ctx, out, msgs)
		if err != nil {
			return err
		}

		msgs = append(msgs,
			&provider.Message{
				Role:    provider.Assistant,
				Content: botMsg,
			},
		)
	}
}

func generate(
	ctx context.Context,
	out io.WriteCloser,
	msgs []*provider.Message,
) (string, error) {
	reader, err := prov.Stream(ctx, cfg, msgs)
	if err != nil {
		return "", err
	}
	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(parser.MultiWriter(out, buf), reader)
	if err != nil {
		return "", err
	}
	out.Write([]byte("\n"))
	return buf.String(), nil
}

// TODO return correct exit code when -run or -exec fails
func runBlock(block *parser.CodeBlock) error {
	switch block.Lang {
	case parser.Go:
		code := block.Code
		// TODO remove when prompt engineering is there to add "make it runnable"
		if !strings.HasPrefix(block.Code, "package") {
			code = fmt.Sprintf("package main\n\n%s", block.Code)
		}
		path := fmt.Sprintf("%smain.go", os.TempDir())
		if err := os.WriteFile(path, []byte(code), 0644); err != nil {
			return err
		}
		if err := runCmd("go", "run", path); err != nil {
			return err
		}
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
	case parser.Python:
		path := fmt.Sprintf("%smain.py", os.TempDir())
		if err := os.WriteFile(path, []byte(block.Code), 0644); err != nil {
			return err
		}
		if err := runCmd("python", path); err != nil {
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
		modelNames, err := prov.ListModels(ctx)
		if err != nil {
			return false, err
		}
		fmt.Println("Available models:")
		for _, model := range modelNames {
			fmt.Println(model)
		}
		fmt.Println()
		fmt.Printf("Currently selected model: %s\n", *cfg.Model)
		return true, nil
	}

	if *listConnectors {
		connectorIDs, err := prov.ListConnectors(ctx)
		if err != nil {
			return false, err
		}
		fmt.Println("Available connectors:")
		for _, connectorID := range connectorIDs {
			fmt.Println(connectorID)
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
	turnFn := func(ctx context.Context, out io.WriteCloser, msgs []*provider.Message) (string, error) {
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

	var pipeContent string
	// Check if there is input from the pipe (stdin)
	if stat, _ := os.Stdin.Stat(); (stat.Mode() & os.ModeCharDevice) == 0 {
		pipeBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from pipe: %w", err)
		}
		pipeContent = string(pipeBytes)
	}
	if flag.NArg() == 0 && pipeContent == "" {
		return fmt.Errorf("what's your command?")
	}
	usrMsg := strings.Join(flag.Args(), " ")
	if pipeContent != "" {
		usrMsg = fmt.Sprintf("%s\n%s", pipeContent, usrMsg)
	}

	var err error
	switch {
	case *chat:
		err = multiTurn(ctx, os.Stdout, os.Stdin, turnFn)
	default:
		_, err = turnFn(ctx, os.Stdout, []*provider.Message{
			{
				Role:    provider.User,
				Content: usrMsg,
			},
		})
	}
	return err
}

func main() {
	flag.Parse()

	ctx := context.Background()
	done, err := parseConfig(ctx)
	if err != nil {
		panic(err)
	}
	if done {
		return
	}

	switch cfg.Provider {
	case config.ProviderGroq:
		prov = provider.NewGroqProvider(os.Getenv(GROQ_API_KEY))
	case config.ProviderCohere:
		prov = provider.NewCohereProvider(os.Getenv(COHERE_API_KEY))
	default:
		log.Fatalf("unknown provider: %s", cfg.Provider)
	}

	if err := cmd(ctx); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
