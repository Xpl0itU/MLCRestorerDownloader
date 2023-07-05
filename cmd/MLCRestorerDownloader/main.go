package main

import (
	"bufio"
	"fmt"
	"os"

	downloader "github.com/Xpl0itU/MLCRestorerDownloader"
)

func main() {
	titles, err := readTitleInfoFromFile("titles.json")
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	fmt.Println("Menu:")
	fmt.Println("1. Download EUR titles")
	fmt.Println("2. Download USA titles")
	fmt.Println("3. Download JPN titles")
	fmt.Println("4. Exit")

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Select an option: ")
	option, err := reader.ReadString('\n')
	if err != nil {
		fmt.Println("Error:", err)
		return
	}

	switch option {
	case "1\n":
		downloadTitles("EUR", titles)
	case "2\n":
		downloadTitles("USA", titles)
	case "3\n":
		downloadTitles("JPN", titles)
	case "4\n":
		fmt.Println("Exiting...")
		return
	default:
		fmt.Println("Invalid option")
		return
	}
}

func downloadTitles(region string, titles TitleMap) {
	selectedRegionTitles := titles[region]
	allRegionTitles := titles["All"]

	allTitles := append(selectedRegionTitles, allRegionTitles...)

	for _, titleID := range allTitles {
		fmt.Printf("Downloading files for title %s on region %s\n", titleID, region)
		downloader.DownloadTitle(titleID, fmt.Sprintf("output/%s/%s", region, titleID))
		fmt.Printf("Download files for title %s on region %s done\n", titleID, region)
	}
}
