package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	downloader "github.com/Xpl0itU/MLCRestorerDownloader"
)

func main() {
	fmt.Println("Menu:")
	fmt.Println("1. Download MLC titles")
	fmt.Println("2. Download SLC titles")
	fmt.Println("3. Exit")

	fmt.Print("Select an option: ")
	var inputKey string
	fmt.Scanln(&inputKey)

	switch inputKey {
	case "1":
		showSubmenu("MLC")
	case "2":
		showSubmenu("SLC")
	case "3":
		fmt.Println("Exiting...")
		return
	default:
		fmt.Println("Invalid option")
		return
	}
}

func showSubmenu(titleType string) {
	if titleType != "MLC" && titleType != "SLC" {
		fmt.Println("Invalid title type")
		return
	}
	titles, err := readTitleInfoFromFile("titles.json")
	if err != nil {
		fmt.Println("[Error]", err)
		return
	}

	var chosenTitles map[string][]string
	switch titleType {
	case "MLC":
		chosenTitles = titles.MLC
	case "SLC":
		chosenTitles = titles.SLC
	default:
		fmt.Println("Invalid option")
		return
	}

	fmt.Println("Menu:")
	fmt.Printf("1. Download EUR %s titles\n", titleType)
	fmt.Printf("2. Download USA %s titles\n", titleType)
	fmt.Printf("3. Download JPN %s titles\n", titleType)
	fmt.Println("4. Back to main menu")

	fmt.Print("Select an option: ")
	var inputKey string
	fmt.Scanln(&inputKey)

	switch inputKey {
	case "1":
		downloadTitles("EUR", chosenTitles, titleType)
	case "2":
		downloadTitles("USA", chosenTitles, titleType)
	case "3":
		downloadTitles("JPN", chosenTitles, titleType)
	case "4":
		fmt.Println("Going back to the main menu...")
		main()
	default:
		fmt.Println("Invalid option")
		return
	}
}

func downloadTitles(region string, titles map[string][]string, titleType string) {
	selectedRegionTitles := titles[region]
	allRegionTitles := titles["All"]

	allTitles := append(selectedRegionTitles, allRegionTitles...)

	commonKey, err := getCommonKey()
	if err != nil {
		fmt.Println("[Error]", err)
		return
	}

	client := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   100,
			MaxConnsPerHost:       100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		},
	}

	progressReporter := NewProgressReporterCLI()

	for _, titleID := range allTitles {
		if titleID == "dummy" {
			continue
		}
		fmt.Printf("\n[Info] Downloading files for title %s on region %s for type %s\n\n", titleID, region, titleType)
		if err := downloader.DownloadTitle(titleID, fmt.Sprintf("output/%s/%s/%s", titleType, region, titleID), progressReporter, client, commonKey); err != nil {
			fmt.Println("[Error]", err)
			os.Exit(1)
		}
		fmt.Printf("\n[Info] Download files for title %s on region %s for type %s done\n\n", titleID, region, titleType)
	}
	fmt.Println("\n[Info] All done!")
}
