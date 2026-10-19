package mlcrestorerdownloader

import (
	"net/http"
	"strings"

	wiiudownloader "github.com/Xpl0itU/WiiUDownloader"
)

type ProgressReporter = wiiudownloader.ProgressReporter

func DownloadTitle(titleID, outputDirectory string, progressReporter ProgressReporter, client *http.Client) error {
	outputDir := strings.TrimRight(outputDirectory, "/\\")

	if client == nil {
		client = http.DefaultClient
	}

	return wiiudownloader.DownloadTitle(
		titleID,
		outputDir,
		wiiudownloader.VersionLatest,
		false,
		progressReporter,
		false,
		client,
		"",
	)
}

func VerifyTitle(titleID, outputDirectory string, progressReporter ProgressReporter) error {
	return DownloadTitle(titleID, outputDirectory, progressReporter, http.DefaultClient)
}
