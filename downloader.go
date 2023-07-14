package mlcrestorerdownloader

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/cavaliergopher/grab/v3"
)

func downloadFile(client *grab.Client, url string, outputPath string) error {
	req, err := grab.NewRequest(outputPath, url)
	if err != nil {
		return err
	}
	resp := client.Do(req)
	if err := resp.Err(); err != nil {
		return err
	}

	fmt.Printf("[Info] Download saved to ./%v \n", resp.Filename)
	return nil
}

func DownloadTitle(titleID string, outputDirectory string, commonKey []byte) error {
	outputDir := strings.TrimRight(outputDirectory, "/\\")
	baseURL := fmt.Sprintf("http://ccs.cdn.c.shop.nintendowifi.net/ccs/download/%s", titleID)
	titleKeyBytes, err := hex.DecodeString(titleID)
	if err != nil {
		return err
	}

	err = os.MkdirAll(outputDir, os.ModePerm)
	if err != nil {
		return err
	}

	client := grab.NewClient()
	downloadURL := fmt.Sprintf("%s/%s", baseURL, "tmd")
	tmdPath := filepath.Join(outputDir, "title.tmd")
	err = downloadFile(client, downloadURL, tmdPath)
	if err != nil {
		return err
	}

	tmdData, err := os.ReadFile(tmdPath)
	if err != nil {
		return err
	}

	var titleVersion uint16
	err = binary.Read(bytes.NewReader(tmdData[476:478]), binary.BigEndian, &titleVersion)
	if err != nil {
		return err
	}

	tikPath := filepath.Join(outputDir, "title.tik")
	downloadURL = fmt.Sprintf("%s/%s", baseURL, "cetk")
	err = downloadFile(client, downloadURL, tikPath)
	if err != nil {
		return err
	}
	tikData, err := os.ReadFile(tikPath)
	if err != nil {
		return err
	}
	encryptedTitleKey := tikData[0x1BF : 0x1BF+0x10]

	var contentCount uint16
	err = binary.Read(bytes.NewReader(tmdData[478:480]), binary.BigEndian, &contentCount)
	if err != nil {
		return err
	}

	cert := bytes.Buffer{}

	cert0, err := getCert(tmdData, 0, contentCount)
	if err != nil {
		return err
	}
	cert.Write(cert0)

	cert1, err := getCert(tmdData, 1, contentCount)
	if err != nil {
		return err
	}
	cert.Write(cert1)

	defaultCert, err := getDefaultCert(client)
	if err != nil {
		return err
	}
	cert.Write(defaultCert)

	certPath := filepath.Join(outputDir, "title.cert")
	certFile, err := os.Create(certPath)
	if err != nil {
		return err
	}
	err = binary.Write(certFile, binary.BigEndian, cert.Bytes())
	if err != nil {
		return err
	}
	defer certFile.Close()

	for i := 0; i < int(contentCount); i++ {
		offset := 2820 + (48 * i)
		var id uint32
		err = binary.Read(bytes.NewReader(tmdData[offset:offset+4]), binary.BigEndian, &id)
		if err != nil {
			return err
		}

		appPath := filepath.Join(outputDir, fmt.Sprintf("%08X.app", id))
		downloadURL = fmt.Sprintf("%s/%08X", baseURL, id)
		err = downloadFile(client, downloadURL, appPath)
		if err != nil {
			return err
		}

		if tmdData[offset+7]&0x2 == 2 {
			h3Path := filepath.Join(outputDir, fmt.Sprintf("%08X.h3", id))
			downloadURL = fmt.Sprintf("%s/%08X.h3", baseURL, id)
			err = downloadFile(client, downloadURL, h3Path)
			if err != nil {
				return err
			}
			var content contentInfo
			content.Hash = tmdData[offset+16 : offset+0x14]
			content.ID = fmt.Sprintf("%08X", id)
			binary.Read(bytes.NewReader(tmdData[offset+8:offset+15]), binary.BigEndian, &content.Size)
			err = checkContentHashes(outputDirectory, commonKey, encryptedTitleKey, titleKeyBytes, content)
			if err != nil {
				return err
			}
		}
	}

	return nil
}
