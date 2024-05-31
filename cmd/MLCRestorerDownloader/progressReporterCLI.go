package main

import (
	"fmt"
	"sync"
	"time"
)

const (
	MAX_SPEEDS       = 32
	SMOOTHING_FACTOR = 0.2
)

type SpeedAverager struct {
	speeds       []int64
	averageSpeed int64
}

func newSpeedAverager() *SpeedAverager {
	return &SpeedAverager{
		speeds:       make([]int64, MAX_SPEEDS),
		averageSpeed: 0,
	}
}

func (sa *SpeedAverager) AddSpeed(speed int64) {
	if len(sa.speeds) >= MAX_SPEEDS {
		copy(sa.speeds[:MAX_SPEEDS/2], sa.speeds[MAX_SPEEDS/2:])
		sa.speeds = sa.speeds[:MAX_SPEEDS/2]
	}
	sa.speeds = append(sa.speeds, speed)
}

func (sa *SpeedAverager) calculateAverageOfSpeeds() {
	var total int64
	for _, speed := range sa.speeds {
		total += speed
	}
	sa.averageSpeed = total / int64(len(sa.speeds))
}

func (sa *SpeedAverager) GetAverageSpeed() float64 {
	sa.calculateAverageOfSpeeds()
	return SMOOTHING_FACTOR*float64(sa.speeds[len(sa.speeds)-1]) + (1-SMOOTHING_FACTOR)*float64(sa.averageSpeed)
}

type ProgressReporterCLI struct {
	totalToDownload int64
	totalDownloaded int64
	progressPerFile map[string]int64 // map of filename to downloaded bytes
	progressMutex   sync.Mutex
	speedAverager   *SpeedAverager
	startTime       time.Time
}

func NewProgressReporterCLI() *ProgressReporterCLI {
	return &ProgressReporterCLI{
		progressPerFile: make(map[string]int64),
		speedAverager:   newSpeedAverager(),
	}
}

func (pr *ProgressReporterCLI) SetGameTitle(title string) {
	fmt.Printf("Downloading %s\n", title)
}

func (pr *ProgressReporterCLI) UpdateDownloadProgress(downloaded int64, filename string) {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	pr.totalDownloaded += downloaded
	pr.progressPerFile[filename] += downloaded
	pr.speedAverager.AddSpeed(downloaded)
}

func (pr *ProgressReporterCLI) UpdateDecryptionProgress(progress float64) {
	fmt.Printf("\rDecryption progress: %.2f%%", progress*100)
}

func (pr *ProgressReporterCLI) SetDownloadSize(size int64) {
	pr.totalToDownload = size
}

func (pr *ProgressReporterCLI) ResetTotals() {
	pr.totalDownloaded = 0
	pr.progressPerFile = make(map[string]int64)
}

func (pr *ProgressReporterCLI) MarkFileAsDone(filename string) {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	pr.progressPerFile[filename] = 0
}

func (pr *ProgressReporterCLI) SetTotalDownloadedForFile(filename string, downloaded int64) {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	pr.progressPerFile[filename] = downloaded
}

func (pr *ProgressReporterCLI) SetStartTime(startTime time.Time) {
	pr.startTime = startTime
}

func (pr *ProgressReporterCLI) PrintProgress() {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	downloadSpeed := pr.speedAverager.GetAverageSpeed()
	percentage := float64(pr.totalDownloaded) / float64(pr.totalToDownload) * 100
	elapsedTime := time.Since(pr.startTime)
	remainingTime := time.Duration(float64(pr.totalToDownload-pr.totalDownloaded) / downloadSpeed * float64(time.Second))
	fmt.Printf("\r%.2f%% downloaded at %.2f MB/s, %s elapsed, %s remaining", percentage, float64(downloadSpeed)/(1024*1024), elapsedTime, remainingTime)
}
