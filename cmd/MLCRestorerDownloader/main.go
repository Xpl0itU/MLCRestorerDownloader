package main

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	downloader "github.com/Xpl0itU/MLCRestorerDownloader"
	"golang.org/x/sync/errgroup"
)

const maxConcurrentTitles = 3

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

func collectUniqueTitles(titles map[string][]string, region string) []string {
	rawList := append(titles[region], titles["All"]...)
	seen := make(map[string]struct{}, len(rawList))
	valid := make([]string, 0, len(rawList))

	for _, tid := range rawList {
		if tid == "dummy" {
			continue
		}
		norm := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(tid), "-", ""))
		if norm == "" {
			continue
		}
		if _, exists := seen[norm]; !exists {
			seen[norm] = struct{}{}
			valid = append(valid, tid)
		}
	}
	return valid
}

func downloadTitles(region string, titles map[string][]string, titleType string) {
	allTitles := collectUniqueTitles(titles, region)
	totalTitles := len(allTitles)

	var (
		failedMu     sync.Mutex
		failedTitles []string
		verifiedCnt  int32
	)

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
	progressReporter.SetBatchTotals(totalTitles)

	fmt.Printf("\n[Info] Starting concurrent download of %d %s titles for region %s (%d titles in parallel)...\n\n",
		totalTitles, titleType, region, maxConcurrentTitles)

	var g errgroup.Group
	g.SetLimit(maxConcurrentTitles)

	for idx, titleID := range allTitles {
		idx := idx
		titleID := titleID
		g.Go(func() error {
			titleReporter := progressReporter.ForTitle(titleID)
			progressReporter.LogLine("[Info] [%d/%d] Downloading title %s (%s %s)", idx+1, totalTitles, titleID, region, titleType)

			targetDir := fmt.Sprintf("output/%s/%s/%s", titleType, region, titleID)
			err := downloader.DownloadTitle(titleID, targetDir, titleReporter, client, commonKey)
			if err != nil {
				titleReporter.Finish(false)
				progressReporter.LogLine("[Error] [%d/%d] Title %s failed: %v", idx+1, totalTitles, titleID, err)
				failedMu.Lock()
				failedTitles = append(failedTitles, titleID)
				failedMu.Unlock()
				return nil
			}

			titleReporter.Finish(true)
			doneNow := atomic.AddInt32(&verifiedCnt, 1)
			progressReporter.LogLine("[Info] [%d/%d] Title %s downloaded and verified OK", doneNow, totalTitles, titleID)
			return nil
		})
	}

	_ = g.Wait()
	progressReporter.ClearLine()

	fmt.Printf("\n[Info] Running final completeness and integrity check on all %d %s (%s) titles...\n", totalTitles, titleType, region)
	var finalMissingOrCorrupt []string
	for idx, titleID := range allTitles {
		targetDir := fmt.Sprintf("output/%s/%s/%s", titleType, region, titleID)
		if err := downloader.VerifyTitle(titleID, targetDir, commonKey, nil); err != nil {
			fmt.Printf("[Error] [%d/%d] Final check FAILED for %s: %v\n", idx+1, totalTitles, titleID, err)
			finalMissingOrCorrupt = append(finalMissingOrCorrupt, titleID)
		}
	}

	fmt.Printf("\n[Info] Final Summary for %s (%s): %d/%d titles present and cryptographically verified OK\n",
		titleType, region, totalTitles-len(finalMissingOrCorrupt), totalTitles)

	if len(finalMissingOrCorrupt) == 0 && len(failedTitles) == 0 {
		fmt.Println("[Info] All done! Every single title is complete and verified.")
		fmt.Println("Press Enter to exit...")
		bufio.NewReader(os.Stdin).ReadBytes('\n')
		os.Exit(0)
	} else {
		fmt.Printf("[Error] Titles failing verification: %v\n", finalMissingOrCorrupt)
		fmt.Println("[Error] Some titles were not able to be downloaded or verified. Would you like to retry?")
		fmt.Println("1. Yes, retry ")
		fmt.Println("2. No, not right now")
		fmt.Print("Select an option: ")
		var inputKey string
		fmt.Scanln(&inputKey)

		switch inputKey {
		case "1":
			downloadTitles(region, titles, titleType)
		default:
			os.Exit(1)
		}
	}
}