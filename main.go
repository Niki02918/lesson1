package main

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	statsURL = "http://srv.msk01.gigacorp.local/_stats"

	loadAvgThreshold      = 30.0
	memUsageThreshold     = 0.8  // 80%
	diskUsageThreshold    = 0.9  // 90%
	netUsageThreshold     = 0.9  // 90%
	bytesInMb        int64 = 1024 * 1024
	bytesInMbit      float64 = 1024 * 1024 // для перевода бит/сек в Мбит/сек
)

func main() {
	// если шаблон в репо ожидает аргументы или иное поведение — это место можно подправить
	_ = filepath.Base(os.Args[0]) // обычно бинарник никак не влияет на логику

	errorCount := 0
	interval := 5 * time.Second // можно поменять, если в задании указан другой интервал

	for {
		messages, ok := pollOnce()
		if !ok {
			errorCount++
			if errorCount >= 3 {
				fmt.Println("Unable to fetch server statistic")
			}
		} else {
			// при успешном запросе логично сбрасывать счётчик ошибок
			errorCount = 0
			for _, msg := range messages {
				fmt.Println(msg)
			}
		}

		time.Sleep(interval)
	}
}

// pollOnce — один опрос сервера: запрос, парсинг, проверки.
// Возвращает список сообщений и признак успешности (ok = true, если данные валидны).
func pollOnce() ([]string, bool) {
	resp, err := http.Get(statsURL)
	if err != nil {
		return nil, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, false
	}

	values, ok := parseStats(strings.TrimSpace(string(body)))
	if !ok {
		return nil, false
	}

	return checkThresholds(values), true
}

// parseStats парсит строку вида "12,2147483648,..."
func parseStats(line string) ([]float64, bool) {
	if line == "" {
		return nil, false
	}
	parts := strings.Split(line, ",")
	if len(parts) != 7 {
		return nil, false
	}

	result := make([]float64, 7)
	for i, p := range parts {
		p = strings.TrimSpace(p)
		v, err := strconv.ParseFloat(p, 64)
		if err != nil {
			return nil, false
		}
		result[i] = v
	}
	return result, true
}

// checkThresholds проверяет значения и генерирует сообщения.
func checkThresholds(values []float64) []string {
	var msgs []string

	loadAvg := values[0]

	memTotal := values[1]
	memUsed := values[2]

	diskTotal := values[3]
	diskUsed := values[4]

	netTotal := values[5]
	netUsed := values[6]

	// 1. Load Average
	if loadAvg > loadAvgThreshold {
		msgs = append(msgs, fmt.Sprintf("Load Average is too high: %.2f", loadAvg))
	}

	// 2. Memory usage
	if memTotal > 0 {
		usage := memUsed / memTotal
		if usage > memUsageThreshold {
			percent := usage * 100
			// округлим до целых вниз — при необходимости можно сменить стратегию
			msgs = append(msgs, fmt.Sprintf("Memory usage too high: %d%%", int(percent)))
		}
	}

	// 3. Disk usage
	if diskTotal > 0 {
		usage := diskUsed / diskTotal
		if usage > diskUsageThreshold {
			freeBytes := int64(diskTotal - diskUsed)
			if freeBytes < 0 {
				freeBytes = 0
			}
			freeMb := freeBytes / bytesInMb
			msgs = append(msgs, fmt.Sprintf("Free disk space is too low: %d Mb left", freeMb))
		}
	}

	// 4. Network bandwidth usage
	if netTotal > 0 {
		usage := netUsed / netTotal
		if usage > netUsageThreshold {
			freeBytesPerSec := netTotal - netUsed
			if freeBytesPerSec < 0 {
				freeBytesPerSec = 0
			}
			// байты -> биты -> мега-биты (через 1024^2)
			freeMbit := (freeBytesPerSec * 8) / bytesInMbit
			msgs = append(msgs, fmt.Sprintf("Network bandwidth usage high: %.0f Mbit/s available", freeMbit))
		}
	}

	return msgs
}
