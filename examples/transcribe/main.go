package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"sync"
	"time"

	"github.com/daulet/cmd/config"
	"github.com/daulet/cmd/provider"
)

const MAX_CONCURRENT_REQUESTS = 10

type work struct {
	idx  int
	file string
}

type result struct {
	idx     int
	content string
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
		prov = cache
	}

	ctx := context.Background()
	var wg sync.WaitGroup
	resCh := make(chan *result)
	workCh := make(chan *work)
	for range MAX_CONCURRENT_REQUESTS {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range workCh {
				for {
					fmt.Println("transcribing", item.file)
					content, err := prov.Transcribe(ctx, &config.Config{}, item.file)
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
						}
						fmt.Printf("waiting for %s\n", waitTime)
						<-time.After(waitTime)
						continue
					}
					resCh <- &result{idx: item.idx, content: content}
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

	transcripts := make([]string, len(files))
	for res := range resCh {
		transcripts[res.idx] = res.content
	}
	wg.Wait()
	close(resCh)

	for idx, transcript := range transcripts {
		fmt.Printf("%d: %s\n", idx, transcript)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
