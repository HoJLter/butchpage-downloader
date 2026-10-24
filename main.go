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
	"strconv"
	"strings"
)

func httpGet(target string) ([]byte, error) {
	slog.Debug("Target=" + target)
	resp, err := http.Get(target)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func writeToFile(path string, data []byte) error {
	outputFile, err := os.OpenFile(
		path,
		os.O_WRONLY|os.O_CREATE,
		0644,
	)
	if err != nil {
		return err
	}
	defer outputFile.Close()

	_, err = outputFile.Write(data)
	if err != nil {
		return err
	}

	return nil
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
	for i, target := range endpoints {
		slog.Debug("Entering the iteration=" + strconv.Itoa(i))
		html, err := httpGet(target)
		if err != nil {
			slog.Error(err.Error())
			continue
		}

		regex, err := regexp.Compile("[a-zA-Z]+")
		if err != nil {
			slog.Error(err.Error())
			continue
		}
		domain := regex.FindAllString(target, 2)[1]
		slog.Debug("Domain=" + domain)
		err = writeToFile(
			*outputDir+"/"+domain+".html",
			html,
		)
		if err != nil {
			slog.Error(err.Error())
			return
		}
	}
}
