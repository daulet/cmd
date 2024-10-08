package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/daulet/cmd/config"
	"github.com/daulet/cmd/parser"
	"github.com/daulet/cmd/provider"

	"github.com/fatih/color"
	"github.com/jessevdk/go-flags"
)

const (
	CONTEXT_TEMPLATE = "%s\n\n%s"

	AUDIO_MIME_PREFIX = "audio/"
	IMAGE_MIME_PREFIX = "image/"
)

var (
	prov provider.Provider
	cfg  *config.Config
)

type flagValues struct {
	Interactive bool `short:"i" long:"interactive" description:"Start chat session with LLM, other flags apply."`
	Execute     bool `short:"e" long:"execute" description:"Execute generated command/code, do not show LLM output."`
	Run         bool `short:"r" long:"run" description:"Stream LLM output and run generated command/code at the end."`
	// TODO support multiple files to allow multiple images
	File *string `short:"f" long:"file" description:"File to process, depending on the type it will either be transcribed or sent as image."`

	ShowConfig bool `short:"c" long:"config" description:"Show current config."`

	ListModels bool    `long:"list-models" description:"List available models."`
	SetModel   *string `long:"model" description:"Set model to use."`

	ListConnectors bool     `long:"list-connectors" description:"List available connectors."`
	SetConnectors  []string `long:"connector" description:"Set connectors to use."`

	SetTemperature      *float64 `short:"t" long:"temperature" description:"Set temperature value."`
	SetTopP             *float64 `short:"p" long:"top-p" description:"Set top-p value."`
	SetTopK             *int     `short:"k" long:"top-k" description:"Set top-k value."`
	SetFrequencyPenalty *float64 `long:"freq" description:"Set frequency penalty value."`
	SetPresencePenalty  *float64 `long:"pres" description:"Set presence penalty value."`
}

func multiTurn(
	ctx context.Context,
	out io.WriteCloser,
	in io.Reader,
	contextMsg *provider.Message,
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

		var nextMsg *provider.Message
		if contextMsg != nil {
			if contextMsg.Content != "" {
				contextMsg.Content = fmt.Sprintf("%s %s", contextMsg.Content, userMsg)
			} else {
				// when first message is multi part, the first part is text
				contextMsg.MultiPart[0] = &provider.MessagePart{
					Field: &provider.TextPart{
						Text: userMsg,
					},
				}
			}
			nextMsg = contextMsg
			contextMsg = nil
		} else {
			nextMsg = &provider.Message{
				Role:    provider.User,
				Content: userMsg,
			}
		}

		msgs = append(msgs, nextMsg)
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

// TODO return correct exit code when -run or -exec fails - is this fixed?
func runBlock(block *parser.CodeBlock, tempDir string) error {
	switch block.Lang {
	case parser.Go:
		code := block.Code
		// TODO remove when prompt engineering is there to add "make it runnable"
		if !strings.HasPrefix(block.Code, "package") {
			code = fmt.Sprintf("package main\n\n%s", block.Code)
		}
		path := fmt.Sprintf("%smain.go", tempDir)
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
		path := fmt.Sprintf("%sindex.html", tempDir)
		if err := os.WriteFile(path, []byte(block.Code), 0644); err != nil {
			return err
		}
		if err := runCmd("open", fmt.Sprintf("file://%s", path)); err != nil {
			return err
		}
	case parser.JavaScript:
		path := fmt.Sprintf("%sscript.js", tempDir) // TODO can this be parsed from model output? sometimes changes
		if err := os.WriteFile(path, []byte(block.Code), 0644); err != nil {
			return err
		}
		// assumed to be part of html, so not executed separately
	case parser.CSS:
		path := fmt.Sprintf("%sstyle.css", tempDir) // TODO can this be parsed from model output? sometimes changes
		if err := os.WriteFile(path, []byte(block.Code), 0644); err != nil {
			return err
		}
		// assumed to be part of html, so not executed separately
	case parser.Python:
		path := fmt.Sprintf("%smain.py", tempDir)
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
	cmd.Stderr = &colorWriter{
		Writer: os.Stderr,
		Color:  color.New(color.FgHiRed),
	}
	return cmd.Run()
}

func parseConfig(ctx context.Context, flagDefs []*flags.Option, flagVals *flagValues) (bool, error) {
	var err error
	cfg, err = config.ReadConfig()
	if err != nil {
		return false, err
	}

	if flagVals.ShowConfig {
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			return false, err
		}
		fmt.Println("Current config:")
		fmt.Println(string(data))
		return true, nil
	}

	if flagVals.ListModels {
		modelNames, err := prov.ListModels(ctx)
		if err != nil {
			return false, err
		}
		fmt.Println("Available models:")
		for _, model := range modelNames {
			fmt.Println(model)
		}
		fmt.Println()
		for modelType, model := range cfg.Model {
			fmt.Printf("Currently selected model for %s: %s\n", modelType, model)
		}
		return true, nil
	}

	if flagVals.ListConnectors {
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
	// TODO changing provider should reset model selection
	if flagVals.SetModel != nil {
		dirtyCfg = true
		model := *flagVals.SetModel
		modelType, err := config.ModelType(model)
		if err != nil {
			return false, err
		}
		// TODO there is no way to unset model
		cfg.Model[modelType] = model
	}

	if flagVals.SetConnectors != nil {
		dirtyCfg = true
		cfg.Connectors = flagVals.SetConnectors
	}

	if flagVals.SetTemperature != nil {
		dirtyCfg = true
		cfg.Temperature = flagVals.SetTemperature
	}

	if flagVals.SetTopP != nil {
		dirtyCfg = true
		cfg.TopP = flagVals.SetTopP
	}

	if flagVals.SetTopK != nil {
		dirtyCfg = true
		cfg.TopK = flagVals.SetTopK
	}

	if flagVals.SetFrequencyPenalty != nil {
		dirtyCfg = true
		cfg.FrequencyPenalty = flagVals.SetFrequencyPenalty
	}

	if flagVals.SetPresencePenalty != nil {
		dirtyCfg = true
		cfg.PresencePenalty = flagVals.SetPresencePenalty
	}

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

func cmd(ctx context.Context, usrMsg string, flagVals *flagValues) error {
	turnFn := func(ctx context.Context, out io.WriteCloser, msgs []*provider.Message) (string, error) {
		var blocks []*parser.CodeBlock
		done := make(chan struct{})

		switch {
		case flagVals.Execute:
			codeW, blockCh := parser.NewCode()
			go func() {
				defer close(done)
				tempDir := os.TempDir()
				for block := range blockCh {
					// TODO the error is swallowed
					_ = runBlock(block, tempDir)
				}
			}()
			// no output to the user, we just execute the code
			out = codeW
		case flagVals.Run:
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

		if out != os.Stdout {
			// has to be closed so we are not blocked on code blocks
			out.Close()
		}
		<-done
		tempDir := os.TempDir()
		for _, block := range blocks {
			if err := runBlock(block, tempDir); err != nil {
				return "", err
			}
		}

		return response, nil
	}

	var (
		in          io.Reader = os.Stdin
		pipeContent string
		err         error
	)
	// Check if there is input from the pipe (stdin)
	if stat, _ := os.Stdin.Stat(); (stat.Mode() & os.ModeCharDevice) == 0 {
		pipeBytes, err := io.ReadAll(os.Stdin)
		if err != nil {
			return fmt.Errorf("failed to read from pipe: %w", err)
		}
		pipeContent = string(pipeBytes)
	}
	if flagVals.Interactive {
		in, err = os.Open("/dev/tty")
		if err != nil {
			return fmt.Errorf("failed to open /dev/tty: %w", err)
		}
	}
	if usrMsg == "" && pipeContent == "" && !flagVals.Interactive && flagVals.File == nil {
		return fmt.Errorf("what's your command?")
	}
	if pipeContent != "" {
		usrMsg = fmt.Sprintf(CONTEXT_TEMPLATE, pipeContent, usrMsg)
	}

	message := &provider.Message{
		Role:    provider.User,
		Content: usrMsg,
	}

	if flagVals.File != nil {
		f, err := os.Open(*flagVals.File)
		if err != nil {
			return fmt.Errorf("failed to open file: %w", err)
		}
		data, err := io.ReadAll(f)
		if err != nil {
			return fmt.Errorf("failed to read file: %w", err)
		}
		contentType := http.DetectContentType(data)

		if strings.HasPrefix(contentType, AUDIO_MIME_PREFIX) {
			segments, err := prov.Transcribe(ctx, cfg, &provider.AudioFile{FilePath: *flagVals.File, Reader: bytes.NewReader(data)})
			if err != nil {
				return fmt.Errorf("failed to transcribe: %w", err)
			}
			var out strings.Builder
			for _, segment := range segments {
				out.WriteString(fmt.Sprintf("%v - %v\n", segment.Start, segment.End))
				out.WriteString(fmt.Sprintf("%s\n", segment.Text))
			}
			os.Stdout.Write([]byte(out.String()))
			return nil
		}

		if strings.HasPrefix(contentType, IMAGE_MIME_PREFIX) {
			message.Content = ""
			message.MultiPart = append(message.MultiPart,
				&provider.MessagePart{
					Field: &provider.TextPart{
						Text: usrMsg,
					},
				},
				&provider.MessagePart{
					Field: &provider.ImagePart{
						Data: fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(data)),
					},
				},
			)
		}
	}

	switch {
	case flagVals.Interactive:
		err = multiTurn(ctx, os.Stdout, in, message, turnFn)
	default:
		_, err = turnFn(ctx, os.Stdout, []*provider.Message{message})
	}
	return err
}

func run() error {
	flagVals := &flagValues{}
	parser := flags.NewParser(nil, flags.Default)
	g, err := parser.AddGroup("Application Options", "", flagVals)
	if err != nil {
		return err
	}
	unparsed, err := parser.Parse()
	if err != nil {
		return err
	}
	usrMsg := strings.Join(unparsed, " ")
	flagDefs := g.Options()

	// read config first so we can use the right provider
	cfg, err := config.ReadConfig()
	if err != nil {
		return err
	}

	switch cfg.Provider {
	case config.ProviderGroq:
		prov, err = provider.NewGroqProvider()
	case config.ProviderCohere:
		prov, err = provider.NewCohereProvider()
	default:
		return fmt.Errorf("unknown provider in config: %s", cfg.Provider)
	}
	if err != nil {
		return err
	}

	ctx := context.Background()
	done, err := parseConfig(ctx, flagDefs, flagVals)
	if err != nil {
		return err
	}
	if done {
		return nil
	}

	if cfg.Record {
		closer, err := provider.NewCacheProvider(prov, ".cache/cache.json")
		if err != nil {
			log.Fatalf("failed to create cache provider: %v", err)
		}
		defer closer.Close()
		prov = closer
	}

	return cmd(ctx, usrMsg, flagVals)
}

func main() {
	err := run()
	if flagsErr, ok := err.(*flags.Error); ok && flagsErr.Type == flags.ErrHelp {
		os.Exit(1)
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		os.Exit(exitErr.ExitCode())
	}
	if err != nil {
		color.Yellow("error: %v\n", err)
		os.Exit(1)
	}
}
