package main

import (
	"fmt"
	"strings"
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
	progressPerFile map[string]int64
	progressMutex   sync.Mutex
	speedAverager   *SpeedAverager
	startTime       time.Time
	lastPrintTime   time.Time
	lastLineLen     int
	totalTitles     int
	completedTitles int
	activeTitles    int
}

func NewProgressReporterCLI() *ProgressReporterCLI {
	return &ProgressReporterCLI{
		progressPerFile: make(map[string]int64),
		speedAverager:   newSpeedAverager(),
		startTime:       time.Now(),
	}
}

func (pr *ProgressReporterCLI) SetBatchTotals(totalTitles int) {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	pr.totalToDownload = 0
	pr.totalDownloaded = 0
	pr.progressPerFile = make(map[string]int64)
	pr.startTime = time.Now()
	pr.lastPrintTime = time.Time{}
	pr.lastLineLen = 0
	pr.totalTitles = totalTitles
	pr.completedTitles = 0
	pr.activeTitles = 0
}

func (pr *ProgressReporterCLI) LogLine(format string, args ...any) {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	if pr.lastLineLen > 0 {
		fmt.Printf("\r%s\r", strings.Repeat(" ", pr.lastLineLen))
		pr.lastLineLen = 0
	}
	fmt.Printf(format+"\n", args...)
}

func (pr *ProgressReporterCLI) ClearLine() {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	if pr.lastLineLen > 0 {
		fmt.Printf("\r%s\r", strings.Repeat(" ", pr.lastLineLen))
		pr.lastLineLen = 0
	}
}

func (pr *ProgressReporterCLI) writeStatusLocked(line string, force bool) {
	now := time.Now()
	if !force && now.Sub(pr.lastPrintTime) < 80*time.Millisecond {
		return
	}
	pr.lastPrintTime = now

	pad := ""
	if len(line) < pr.lastLineLen {
		pad = strings.Repeat(" ", pr.lastLineLen-len(line))
	}
	pr.lastLineLen = len(line)
	fmt.Printf("\r%s%s", line, pad)
}

func (pr *ProgressReporterCLI) renderProgressLocked(force bool) {
	elapsed := time.Since(pr.startTime)
	elapsedSec := elapsed.Seconds()
	var speedMBps float64
	if elapsedSec > 0.05 {
		speedMBps = (float64(pr.totalDownloaded) / (1024 * 1024)) / elapsedSec
	}

	var percentage float64
	if pr.totalToDownload > 0 {
		percentage = float64(pr.totalDownloaded) / float64(pr.totalToDownload) * 100
		if percentage > 100 {
			percentage = 100
		}
	}

	var remaining time.Duration
	if speedMBps > 0.001 && pr.totalToDownload > pr.totalDownloaded {
		remBytes := float64(pr.totalToDownload - pr.totalDownloaded)
		remSec := remBytes / (speedMBps * 1024 * 1024)
		remaining = time.Duration(remSec * float64(time.Second)).Round(100 * time.Millisecond)
	}

	elapsedRounded := elapsed.Round(100 * time.Millisecond)

	var line string
	if pr.totalTitles > 0 {
		line = fmt.Sprintf("[%d/%d done | %d active] %.2f%% downloaded at %.2f MB/s, %s elapsed, %s remaining",
			pr.completedTitles, pr.totalTitles, pr.activeTitles, percentage, speedMBps, elapsedRounded, remaining)
	} else {
		line = fmt.Sprintf("%.2f%% downloaded at %.2f MB/s, %s elapsed, %s remaining",
			percentage, speedMBps, elapsedRounded, remaining)
	}

	pr.writeStatusLocked(line, force)
}

func (pr *ProgressReporterCLI) SetGameTitle(title string) {
	pr.LogLine("Downloading %s", title)
}

func (pr *ProgressReporterCLI) UpdateDownloadProgress(downloaded int64, filename string) {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	pr.totalDownloaded += downloaded
	pr.progressPerFile[filename] += downloaded
	pr.speedAverager.AddSpeed(downloaded)
}

func (pr *ProgressReporterCLI) UpdateDecryptionProgress(progress float64) {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	line := fmt.Sprintf("Verification progress: %.2f%%", progress*100)
	pr.writeStatusLocked(line, progress >= 1.0)
}

func (pr *ProgressReporterCLI) SetDownloadSize(size int64) {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	pr.totalToDownload = size
}

func (pr *ProgressReporterCLI) ResetTotals() {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
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
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	if pr.startTime.IsZero() {
		pr.startTime = startTime
	}
}

func (pr *ProgressReporterCLI) PrintProgress() {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	pr.renderProgressLocked(false)
}

type TitleProgressReporter struct {
	parent        *ProgressReporterCLI
	titleID       string
	mu            sync.Mutex
	allocatedSize int64
	countedBytes  int64
	fileBytes     map[string]int64
}

func (pr *ProgressReporterCLI) ForTitle(titleID string) *TitleProgressReporter {
	pr.progressMutex.Lock()
	pr.activeTitles++
	pr.progressMutex.Unlock()
	return &TitleProgressReporter{
		parent:    pr,
		titleID:   titleID,
		fileBytes: make(map[string]int64),
	}
}

func (tr *TitleProgressReporter) Finish(success bool) {
	tr.parent.progressMutex.Lock()
	if tr.parent.activeTitles > 0 {
		tr.parent.activeTitles--
	}
	if success {
		tr.parent.completedTitles++
	}
	tr.parent.renderProgressLocked(true)
	tr.parent.progressMutex.Unlock()
}

func (tr *TitleProgressReporter) SetGameTitle(title string) {}

func (tr *TitleProgressReporter) UpdateDownloadProgress(downloaded int64, filename string) {
	tr.mu.Lock()
	tr.fileBytes[filename] += downloaded
	tr.countedBytes += downloaded
	tr.mu.Unlock()

	tr.parent.progressMutex.Lock()
	tr.parent.totalDownloaded += downloaded
	tr.parent.speedAverager.AddSpeed(downloaded)
	tr.parent.progressMutex.Unlock()
}

func (tr *TitleProgressReporter) UpdateDecryptionProgress(progress float64) {
	tr.parent.progressMutex.Lock()
	tr.parent.renderProgressLocked(false)
	tr.parent.progressMutex.Unlock()
}

func (tr *TitleProgressReporter) SetDownloadSize(size int64) {
	tr.mu.Lock()
	diff := size - tr.allocatedSize
	tr.allocatedSize = size
	tr.mu.Unlock()

	tr.parent.progressMutex.Lock()
	tr.parent.totalToDownload += diff
	tr.parent.renderProgressLocked(false)
	tr.parent.progressMutex.Unlock()
}

func (tr *TitleProgressReporter) ResetTotals() {
	tr.mu.Lock()
	prevCounted := tr.countedBytes
	prevAlloc := tr.allocatedSize
	tr.countedBytes = 0
	tr.allocatedSize = 0
	tr.fileBytes = make(map[string]int64)
	tr.mu.Unlock()

	tr.parent.progressMutex.Lock()
	tr.parent.totalDownloaded -= prevCounted
	if tr.parent.totalDownloaded < 0 {
		tr.parent.totalDownloaded = 0
	}
	tr.parent.totalToDownload -= prevAlloc
	if tr.parent.totalToDownload < 0 {
		tr.parent.totalToDownload = 0
	}
	tr.parent.progressMutex.Unlock()
}

func (tr *TitleProgressReporter) MarkFileAsDone(filename string) {
	tr.mu.Lock()
	delete(tr.fileBytes, filename)
	tr.mu.Unlock()
}

func (tr *TitleProgressReporter) SetTotalDownloadedForFile(filename string, downloaded int64) {
	tr.mu.Lock()
	prev := tr.fileBytes[filename]
	diff := downloaded - prev
	tr.fileBytes[filename] = downloaded
	tr.countedBytes += diff
	tr.mu.Unlock()

	if diff != 0 {
		tr.parent.progressMutex.Lock()
		tr.parent.totalDownloaded += diff
		if tr.parent.totalDownloaded < 0 {
			tr.parent.totalDownloaded = 0
		}
		tr.parent.progressMutex.Unlock()
	}
}

func (tr *TitleProgressReporter) SetStartTime(startTime time.Time) {
	tr.parent.SetStartTime(startTime)
}

func (tr *TitleProgressReporter) PrintProgress() {
	tr.parent.PrintProgress()
}