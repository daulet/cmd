package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/daulet/cmd/config"
	"github.com/daulet/cmd/provider"
)

const MAX_CONCURRENT_REQUESTS = 2

type work struct {
	idx  int
	file string
}

type result struct {
	idx      int
	segments []*provider.AudioSegment
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("Usage: transcribe <directory>")
	}
	dir := os.Args[1]

	files, err := filepath.Glob(filepath.Join(dir, "*.mp3"))
	if err != nil {
		return fmt.Errorf("failed to glob files: %w", err)
	}

	prov := provider.NewGroqProvider(os.Getenv("GROQ_API_KEY"))
	{
		cache, err := provider.NewCacheProvider(prov, ".cache/cache.json")
		if err != nil {
			return fmt.Errorf("failed to create cache provider: %w", err)
		}
		defer cache.Close()

		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt)
		go func() {
			<-sigCh
			cache.Close()
			os.Exit(1)
		}()

		prov = cache
	}

	var (
		wg     sync.WaitGroup
		ctx    = context.Background()
		workCh = make(chan *work)
		resCh  = make(chan *result)
	)
	for range MAX_CONCURRENT_REQUESTS {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range workCh {
				data, err := os.ReadFile(item.file)
				if err != nil {
					panic(err)
				}
				for {
					fmt.Println("transcribing", item.file)
					segments, err := prov.Transcribe(ctx, &config.Config{}, &provider.AudioFile{
						FilePath: item.file,
						Reader:   bytes.NewReader(data),
					})
					if err != nil {
						waitTime := time.Minute
						// Parse error message like:
						// "Please try again in 6m6.125s."
						re := regexp.MustCompile(`Please try again in (\d+)m(\d+.\d+)s\.`)
						matches := re.FindStringSubmatch(err.Error())
						if len(matches) == 3 {
							minutes, _ := strconv.Atoi(matches[1])
							seconds, _ := strconv.ParseFloat(matches[2], 64)
							waitTime = time.Duration(minutes)*time.Minute + time.Duration(seconds)*time.Second
						} else {
							fmt.Printf("failed to parse error: %w\n", err)
						}
						fmt.Printf("waiting for %s\n", waitTime)
						<-time.After(waitTime)
						continue
					}
					resCh <- &result{idx: item.idx, segments: segments}
					break
				}
			}
		}()
	}

	go func() {
		for idx, file := range files {
			workCh <- &work{idx: idx, file: file}
		}
		close(workCh)
	}()

	transcripts := make([][]*provider.AudioSegment, len(files))
	for range files {
		res := <-resCh
		transcripts[res.idx] = res.segments
	}
	wg.Wait()
	close(resCh)

	for idx, transcript := range transcripts {
		fmt.Printf("File %d:\n", idx)
		for _, segment := range transcript {
			fmt.Printf("%v - %v\n", segment.Start, segment.End)
			fmt.Printf("%s\n", segment.Text)
		}
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
