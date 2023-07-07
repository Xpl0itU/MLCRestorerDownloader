package main

import (
	"bufio"
	"fmt"
	"os"
	"sync"

	downloader "github.com/Xpl0itU/MLCRestorerDownloader"
)

func main() {
	fmt.Println("Menu:")
	fmt.Println("1. Download MLC titles")
	fmt.Println("2. Download SLC titles")
	fmt.Println("3. Exit")

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Select an option: ")
	option, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	switch option {
	case "1\n":
		showSubmenu("MLC")
	case "2\n":
		showSubmenu("SLC")
	case "3\n":
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
		fmt.Println("Error:", err)
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

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Select an option: ")
	option, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	switch option {
	case "1\n":
		downloadTitles("EUR", chosenTitles, titleType)
	case "2\n":
		downloadTitles("USA", chosenTitles, titleType)
	case "3\n":
		downloadTitles("JPN", chosenTitles, titleType)
	case "4\n":
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

	var wg sync.WaitGroup
	for _, titleID := range allTitles {
		if titleID == "dummy" {
			continue
		}
		wg.Add(1)
		go func(tID, rgn, tType string) {
			defer wg.Done()
			fmt.Printf("Downloading files for title %s on region %s for type %s\n", tID, rgn, tType)
			err := downloader.DownloadTitle(tID, fmt.Sprintf("output/%s/%s/%s", tType, rgn, tID))
			if err != nil {
				fmt.Println("Error:", err)
				os.Exit(1)
			}
			fmt.Printf("Download files for title %s on region %s for type %s done\n", tID, rgn, tType)
		}(titleID, region, titleType)
	}
	wg.Wait()
	fmt.Println("All done!")
}
