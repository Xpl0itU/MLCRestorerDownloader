package mlcrestorerdownloader

import (
	"encoding/binary"
	"fmt"
	"io"
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

func GetTitleSize(titleID string, client *http.Client) (int64, error) {
	if client == nil {
		client = http.DefaultClient
	}
	url := fmt.Sprintf("http://ccs.cdn.c.shop.nintendowifi.net/ccs/download/%s/tmd", titleID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return 0, err
	}
	req.Header.Set("User-Agent", "wiiu libcurl/7.88.1-DEV")

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	return ParseTMDSize(data)
}

func ParseTMDSize(data []byte) (int64, error) {
	if len(data) < 0x204 {
		return 0, fmt.Errorf("tmd too short (%d bytes)", len(data))
	}

	sigType := binary.BigEndian.Uint32(data[0:4])
	var sigLen int
	switch sigType {
	case 0x00010000, 0x00010003:
		sigLen = 512
	case 0x00010001, 0x00010004:
		sigLen = 256
	case 0x00010002, 0x00010005:
		sigLen = 64
	default:
		sigLen = 256
	}

	mainHeaderOffset := 4 + sigLen + 60
	if len(data) < mainHeaderOffset+0xA4 {
		return 0, fmt.Errorf("tmd truncated in main header")
	}

	version := data[mainHeaderOffset+64]
	numContents := int(binary.BigEndian.Uint16(data[mainHeaderOffset+64+0x5E : mainHeaderOffset+64+0x60]))

	var recordsOffset int
	var recordSize int

	if version == 1 {
		recordsOffset = mainHeaderOffset + 0xA4 + 32 + (64 * 36)
		recordSize = 0x30
	} else {
		recordsOffset = mainHeaderOffset + 0xA4
		recordSize = 0x24
	}

	if len(data) < recordsOffset+(numContents*recordSize) {
		return 0, fmt.Errorf("tmd truncated before content records (expected at least %d bytes, got %d)",
			recordsOffset+(numContents*recordSize), len(data))
	}

	var totalSize int64
	for i := 0; i < numContents; i++ {
		rec := recordsOffset + (i * recordSize)
		cSize := binary.BigEndian.Uint64(data[rec+8 : rec+16])
		totalSize += int64(cSize)
	}

	return totalSize, nil
}
