package mlcrestorerdownloader

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func downloadFile(url string, outputPath string) error {
	response, err := http.Get(url)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file from %s, status code: %d", url, response.StatusCode)
	}

	out, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, response.Body)
	return err
}

func DownloadTitle(titleID string, outputDirectory string) error {
	outputDir := strings.TrimRight(outputDirectory, "/\\")
	baseURL := fmt.Sprintf("http://ccs.cdn.c.shop.nintendowifi.net/ccs/download/%s", titleID)

	err := os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		return err
	}

	downloadURL := fmt.Sprintf("%s/%s", baseURL, "tmd")
	resp, err := http.Get(downloadURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download tmd from %s, status code: %d", downloadURL, resp.StatusCode)
	}

	tmdData := bytes.Buffer{}
	_, err = io.Copy(&tmdData, resp.Body)
	if err != nil {
		return err
	}

	tmdPath := filepath.Join(outputDir, "title.tmd")
	tmdFile, err := os.Create(tmdPath)
	if err != nil {
		return err
	}
	defer tmdFile.Close()

	_, err = tmdFile.Write(tmdData.Bytes())
	if err != nil {
		return err
	}

	var titleVersion uint16
	err = binary.Read(bytes.NewReader(tmdData.Bytes()[476:478]), binary.BigEndian, &titleVersion)
	if err != nil {
		return err
	}

	tikPath := filepath.Join(outputDir, "title.tik")
	downloadURL = fmt.Sprintf("%s/%s", baseURL, "cetk")
	err = downloadFile(downloadURL, tikPath)
	if err != nil {
		return err
	}

	var contentCount uint16
	err = binary.Read(bytes.NewReader(tmdData.Bytes()[478:480]), binary.BigEndian, &contentCount)
	if err != nil {
		return err
	}

	fmt.Println("Generating our own title.cert...")
	cetk := bytes.Buffer{}
	defaultCert, err := getDefaultCert()
	if err != nil {
		return err
	}
	cetk.Write(getCert(&tmdData, 0, contentCount))
	cetk.Write(getCert(&tmdData, 1, contentCount))
	cetk.Write(defaultCert)

	certPath := filepath.Join(outputDir, "title.cert")
	certFile, err := os.Create(certPath)
	if err != nil {
		return err
	}
	err = binary.Write(certFile, binary.BigEndian, cetk.Bytes())
	if err != nil {
		return err
	}
	certFile.Close()

	for i := 0; i < int(contentCount); i++ {
		offset := 2820 + (48 * i)
		var id uint32
		err = binary.Read(bytes.NewReader(tmdData.Bytes()[offset:offset+4]), binary.BigEndian, &id)
		if err != nil {
			return err
		}

		appPath := filepath.Join(outputDir, fmt.Sprintf("%08X.app", id))
		downloadURL = fmt.Sprintf("%s/%08X", baseURL, id)
		err = downloadFile(downloadURL, appPath)
		if err != nil {
			return err
		}

		if tmdData.Bytes()[offset+7]&0x2 == 2 {
			h3Path := filepath.Join(outputDir, fmt.Sprintf("%08X.h3", id))
			downloadURL = fmt.Sprintf("%s/%08X.h3", baseURL, id)
			err = downloadFile(downloadURL, h3Path)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
