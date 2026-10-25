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
)

type HttpResult struct {
	url  string
	data []byte
	err  error
}

func httpGetWorker(wg *sync.WaitGroup, url string, resCh chan<- HttpResult) {
	defer wg.Done()
	slog.Debug("HttpGet function called", "target", url)

	resp, err := http.Get(url)
	if err != nil {
		resCh <- HttpResult{url, nil, err}
		return
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		resCh <- HttpResult{url, nil, err}
		return
	}
	resCh <- HttpResult{url, data, nil}
}

func writeToFileWorker(
	path string,
	wg *sync.WaitGroup,
	data []byte,
	errCh chan<- error,
) {
	defer wg.Done()
	outputFile, err := os.OpenFile(
		path,
		os.O_WRONLY|os.O_CREATE,
		0644,
	)
	if err != nil {
		errCh <- err
	}
	defer outputFile.Close()

	_, err = outputFile.Write(data)
	if err != nil {
		errCh <- err
	}

	errCh <- nil
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

	err := os.Mkdir(*outputDir, 0644)
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
	for job := range jobsChan {
		wg.Add(1)
		go httpGetWorker(&wg, job, resCh)
	}
	wg.Wait()
	close(resCh)

	writeErrCh := make(chan error, len(endpoints))
	for res := range resCh {
		if res.err != nil {
			slog.Error(err.Error())
			continue
		}

		domain, err := getDomain(res.url)
		if err != nil {
			slog.Error(err.Error())
			continue
		}
		go writeToFileWorker(
			*outputDir+domain+".html",
			&wg,
			res.data,
			writeErrCh,
		)

	}

}
