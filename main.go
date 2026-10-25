package main

import (
	"errors"
	"flag"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

var httpClient = http.Client{
	Timeout: 5 * time.Second,
}

const workersCount = 30

func httpToFileWorker(
	wg *sync.WaitGroup,
	url string,
	outputPath string,
	semaphore chan struct{}) {

	defer wg.Done()
	defer func() { <-semaphore }()
	semaphore <- struct{}{}

	slog.Debug("httpToFile function called", "target", url)

	resp, err := httpClient.Get(url)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	defer resp.Body.Close()

	file, err := os.OpenFile(outputPath, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	defer file.Close()

	_, err = io.Copy(file, resp.Body)
	if err != nil {
		slog.Error(err.Error())
		return
	}
}

func getDomain(rawUrl string) (string, error) {
	u, err := url.Parse(rawUrl)
	if err != nil {
		slog.Error(err.Error())
		return "", err
	}
	return u.Hostname(), nil
}

func configureLogger() {
	opts := slog.HandlerOptions{
		AddSource: false,
		Level:     slog.LevelError,
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &opts))
	slog.SetDefault(logger)
}

func main() {
	configureLogger()

	inputFilename := flag.String("i", "input.txt", "input filename")
	outputDir := flag.String("o", "out", "out directory")

	flag.Parse()

	err := os.Mkdir(*outputDir, 0755)
	if err != nil && !errors.Is(err, os.ErrExist) {
		slog.Error(err.Error())
		return
	}

	inputFile, err := os.OpenFile(*inputFilename, os.O_RDONLY, 0644)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	defer inputFile.Close()

	input, err := io.ReadAll(inputFile)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	endpoints := strings.Split(string(input), ";")

	wg := sync.WaitGroup{}
	semaphore := make(chan struct{}, workersCount)

	for _, url := range endpoints {
		domain, err := getDomain(url)
		if err != nil {
			slog.Error(err.Error())
			continue
		}

		wg.Add(1)
		go httpToFileWorker(
			&wg,
			url,
			*outputDir+"/"+domain+".html",
			semaphore,
		)
	}
	wg.Wait()
}
