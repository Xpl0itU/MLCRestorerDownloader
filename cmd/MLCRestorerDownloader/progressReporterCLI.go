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
	BAR_WIDTH        = 20
)

type SpeedAverager struct {
	speeds       []int64
	averageSpeed int64
}

func newSpeedAverager() *SpeedAverager {
	return &SpeedAverager{
		speeds:       make([]int64, 0, MAX_SPEEDS),
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
	if len(sa.speeds) == 0 {
		sa.averageSpeed = 0
		return
	}
	var total int64
	for _, speed := range sa.speeds {
		total += speed
	}
	sa.averageSpeed = total / int64(len(sa.speeds))
}

func (sa *SpeedAverager) GetAverageSpeed() float64 {
	if len(sa.speeds) == 0 {
		return 0
	}
	sa.calculateAverageOfSpeeds()
	return SMOOTHING_FACTOR*float64(sa.speeds[len(sa.speeds)-1]) + (1-SMOOTHING_FACTOR)*float64(sa.averageSpeed)
}

type SpeedEstimator struct {
	lastTime  time.Time
	lastBytes int64
	speedMBps float64
}

func newSpeedEstimator() *SpeedEstimator {
	return &SpeedEstimator{
		lastTime: time.Now(),
	}
}

func (se *SpeedEstimator) Update(currentBytes int64) float64 {
	now := time.Now()
	dt := now.Sub(se.lastTime).Seconds()
	if dt >= 0.25 {
		diff := currentBytes - se.lastBytes
		if diff < 0 {
			diff = 0
		}
		instant := (float64(diff) / (1024 * 1024)) / dt
		if se.speedMBps == 0 {
			se.speedMBps = instant
		} else {
			se.speedMBps = 0.3*instant + 0.7*se.speedMBps
		}
		se.lastTime = now
		se.lastBytes = currentBytes
	}
	return se.speedMBps
}

type ProgressReporterCLI struct {
	totalToDownload int64
	totalDownloaded int64
	progressPerFile map[string]int64
	progressMutex   sync.Mutex
	speedAverager   *SpeedAverager
	speedEstimator  *SpeedEstimator
	startTime       time.Time
	lastPrintTime   time.Time
	totalTitles     int
	completedTitles int
	activeTitles    int
	activeReporters map[*TitleProgressReporter]struct{}
}

func NewProgressReporterCLI() *ProgressReporterCLI {
	return &ProgressReporterCLI{
		progressPerFile: make(map[string]int64),
		speedAverager:   newSpeedAverager(),
		speedEstimator:  newSpeedEstimator(),
		startTime:       time.Now(),
		activeReporters: make(map[*TitleProgressReporter]struct{}),
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
	pr.totalTitles = totalTitles
	pr.completedTitles = 0
	pr.activeTitles = 0
	pr.speedEstimator = newSpeedEstimator()
	pr.activeReporters = make(map[*TitleProgressReporter]struct{})
}

func (pr *ProgressReporterCLI) LogLine(format string, args ...any) {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	fmt.Print("\r\033[K")
	fmt.Printf(format+"\n", args...)
}

func (pr *ProgressReporterCLI) ClearLine() {
	pr.progressMutex.Lock()
	defer pr.progressMutex.Unlock()
	fmt.Print("\r\033[K")
}

func (pr *ProgressReporterCLI) writeStatusLocked(line string, force bool) {
	now := time.Now()
	if !force && now.Sub(pr.lastPrintTime) < 80*time.Millisecond {
		return
	}
	pr.lastPrintTime = now
	fmt.Printf("\r\033[K%s", line)
}

func formatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	s := int(d.Seconds())
	h := s / 3600
	m := (s % 3600) / 60
	sec := s % 60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%02d:%02d", m, sec)
}

func makeProgressBar(percentage float64, width int) string {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}
	filled := int((percentage / 100.0) * float64(width))
	if filled > width {
		filled = width
	}
	return strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
}

func (pr *ProgressReporterCLI) renderProgressLocked(force bool) {
	elapsed := time.Since(pr.startTime)
	speedMBps := pr.speedEstimator.Update(pr.totalDownloaded)
	if speedMBps <= 0 && elapsed.Seconds() > 0.1 {
		speedMBps = (float64(pr.totalDownloaded) / (1024 * 1024)) / elapsed.Seconds()
	}

	var percentage float64
	var remainingStr string = "--:--"

	if pr.totalTitles > 0 {
		var activeFraction float64
		for rep := range pr.activeReporters {
			rep.mu.Lock()
			if rep.allocatedSize > 0 {
				frac := float64(rep.countedBytes) / float64(rep.allocatedSize)
				if frac > 1.0 {
					frac = 1.0
				}
				activeFraction += frac
			}
			rep.mu.Unlock()
		}

		effectiveDone := float64(pr.completedTitles) + activeFraction
		percentage = (effectiveDone / float64(pr.totalTitles)) * 100.0
		if percentage > 100.0 {
			percentage = 100.0
		}

		totalSizeStr := "?"
		if percentage >= 2.0 && pr.totalDownloaded > 0 {
			estTotal := int64(float64(pr.totalDownloaded) / (percentage / 100.0))
			totalSizeStr = "~" + formatBytes(estTotal)
		} else if pr.totalToDownload > 0 {
			totalSizeStr = formatBytes(pr.totalToDownload)
		}

		if effectiveDone >= 0.2 && elapsed.Seconds() > 1.0 {
			titlesPerSec := effectiveDone / elapsed.Seconds()
			remTitles := float64(pr.totalTitles) - effectiveDone
			if remTitles > 0 && titlesPerSec > 0 {
				remSec := remTitles / titlesPerSec
				remainingStr = formatDuration(time.Duration(remSec * float64(time.Second)))
			} else {
				remainingStr = "00:00"
			}
		}

		line := fmt.Sprintf("[%d/%d done | %d active] [%s] %5.1f%% | %s / %s | %.2f MB/s | %s < %s",
			pr.completedTitles,
			pr.totalTitles,
			pr.activeTitles,
			makeProgressBar(percentage, BAR_WIDTH),
			percentage,
			formatBytes(pr.totalDownloaded),
			totalSizeStr,
			speedMBps,
			formatDuration(elapsed),
			remainingStr,
		)
		pr.writeStatusLocked(line, force)

	} else {
		if pr.totalToDownload > 0 {
			percentage = float64(pr.totalDownloaded) / float64(pr.totalToDownload) * 100.0
			if percentage > 100.0 {
				percentage = 100.0
			}
			if speedMBps > 0.01 && pr.totalToDownload > pr.totalDownloaded {
				remBytes := float64(pr.totalToDownload - pr.totalDownloaded)
				remSec := remBytes / (speedMBps * 1024 * 1024)
				remainingStr = formatDuration(time.Duration(remSec * float64(time.Second)))
			}
		}

		line := fmt.Sprintf("[%s] %5.1f%% | %s / %s | %.2f MB/s | %s < %s",
			makeProgressBar(percentage, BAR_WIDTH),
			percentage,
			formatBytes(pr.totalDownloaded),
			formatBytes(pr.totalToDownload),
			speedMBps,
			formatDuration(elapsed),
			remainingStr,
		)
		pr.writeStatusLocked(line, force)
	}
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
	line := fmt.Sprintf("Verifying: [%s] %5.1f%%", makeProgressBar(progress*100, BAR_WIDTH), progress*100)
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
	tr := &TitleProgressReporter{
		parent:    pr,
		titleID:   titleID,
		fileBytes: make(map[string]int64),
	}
	pr.activeReporters[tr] = struct{}{}
	pr.progressMutex.Unlock()
	return tr
}

func (tr *TitleProgressReporter) Finish(success bool) {
	tr.parent.progressMutex.Lock()
	delete(tr.parent.activeReporters, tr)
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
	tr.parent.renderProgressLocked(false)
	tr.parent.progressMutex.Unlock()
}

func (tr *TitleProgressReporter) UpdateDecryptionProgress(progress float64) {
	tr.parent.progressMutex.Lock()
	line := fmt.Sprintf("[%s] Verifying: [%s] %5.1f%%", tr.titleID, makeProgressBar(progress*100, 15), progress*100)
	tr.parent.writeStatusLocked(line, progress >= 1.0)
	tr.parent.progressMutex.Unlock()
}

func (tr *TitleProgressReporter) Cancelled() bool {
	return false
}

func (tr *TitleProgressReporter) SetCancelled() {}

func (tr *TitleProgressReporter) Done() <-chan struct{} {
	return nil
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
