package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"
)

type HttpResult struct {
	url  string
	body io.ReadCloser
	err  error
}

var httpClient = http.Client{
	Timeout: 5 * time.Second,
}

const workersCount = 5

func httpGetWorker(wg *sync.WaitGroup, url string, resCh chan<- HttpResult, semaphore chan struct{}) {
	defer wg.Done()
	defer func() { <-semaphore }()
	semaphore <- struct{}{}

	slog.Debug("HttpGet function called", "target", url)

	resp, err := httpClient.Get(url)
	if err != nil {
		resCh <- HttpResult{url, nil, err}
		return
	}
	resCh <- HttpResult{url, resp.Body, nil}
}

func writeToFileWorker(
	path string,
	wg *sync.WaitGroup,
	body io.ReadCloser,
) {
	defer wg.Done()
	defer body.Close()
	slog.Debug("Writing", "path", path)

	outputFile, err := os.OpenFile(
		path,
		os.O_WRONLY|os.O_CREATE|os.O_TRUNC,
		0644,
	)
	if err != nil {
		slog.Error(err.Error())
		return
	}
	defer outputFile.Close()

	data, err := io.ReadAll(body)
	if err != nil {
		slog.Error(err.Error())
		return
	}

	_, err = outputFile.Write(data)
	if err != nil {
		slog.Error(err.Error())
		return
	}
}

func getDomain(url string) (string, error) {
	regex, err := regexp.Compile("[a-zA-Z]+")
	if err != nil {
		slog.Error(err.Error())
		return "", err
	}
	domain := regex.FindAllString(url, 2)[1]
	return domain, nil
}

func configureLogger() {
	opts := slog.HandlerOptions{
		AddSource: true,
		Level:     slog.LevelDebug,
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
	fmt.Println(endpoints)

	wg := sync.WaitGroup{}
	jobsChan := make(chan string, len(endpoints))
	for _, endpoint := range endpoints {
		jobsChan <- endpoint
	}
	close(jobsChan)

	resCh := make(chan HttpResult, len(endpoints))
	semaphore := make(chan struct{}, workersCount)

	for job := range jobsChan {
		wg.Add(1)
		go httpGetWorker(&wg, job, resCh, semaphore)
	}
	wg.Wait()
	close(resCh)

	for res := range resCh {
		if res.err != nil {
			slog.Error(res.err.Error())
			continue
		}

		domain, err := getDomain(res.url)
		if err != nil {
			slog.Error(err.Error())
			continue
		}
		wg.Add(1)

		go writeToFileWorker(
			*outputDir+"/"+domain+".html",
			&wg,
			res.body,
		)
	}
	wg.Wait()
}
