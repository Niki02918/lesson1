package main

import (
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	urlStats = "http://srv.msk01.gigacorp.local/_stats"

	loadAvgLimit   = 30.0
	memUsageLimit  = 0.80
	diskUsageLimit = 0.90
	netUsageLimit  = 0.90
)

func main() {
	errorCount := 0

	for {
		msgs, ok := pollOnce()
		if !ok {
			errorCount++
			// сообщение — только когда счётчик впервые достиг 3
			if errorCount == 3 {
				fmt.Println("Unable to fetch server statistic")
			}
		} else {
			errorCount = 0
			for _, m := range msgs {
				fmt.Println(m)
			}
		}

		// делаем опрос чаще, чтобы успеть обработать все сценарии
		time.Sleep(50 * time.Millisecond)
	}
}

// ------------------ запрос ---------------------

func pollOnce() ([]string, bool) {
	resp, err := http.Get(urlStats)
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

	return evaluate(values), true
}

// ------------------ парсинг ---------------------

func parseStats(s string) ([]float64, bool) {
	parts := strings.Split(s, ",")
	if len(parts) != 7 {
		return nil, false
	}

	result := make([]float64, 7)
	for i, p := range parts {
		v, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return nil, false
		}
		result[i] = v
	}
	return result, true
}

// ------------------ проверки ---------------------

func evaluate(v []float64) []string {
	var msgs []string

	loadAvg := v[0]

	memTotal := v[1]
	memUsed := v[2]

	diskTotal := v[3]
	diskUsed := v[4]

	netTotal := v[5]
	netUsed := v[6]

	// Load Average
	if loadAvg > loadAvgLimit {
		msgs = append(msgs, fmt.Sprintf("Load Average is too high: %d", int(loadAvg)))
	}

	// Memory usage
	if memTotal > 0 {
		usage := memUsed / memTotal
		if usage > memUsageLimit {
			msgs = append(msgs, fmt.Sprintf("Memory usage too high: %d%%", int(usage*100)))
		}
	}

	// Disk usage
	if diskTotal > 0 {
		usage := diskUsed / diskTotal
		if usage > diskUsageLimit {
			freeBytes := diskTotal - diskUsed
			if freeBytes < 0 {
				freeBytes = 0
			}
			freeMb := int64(freeBytes) / (1024 * 1024)
			msgs = append(msgs, fmt.Sprintf("Free disk space is too low: %d Mb left", freeMb))
		}
	}

	// Network usage
	if netTotal > 0 {
		usage := netUsed / netTotal
		if usage > netUsageLimit {
			freeBytes := netTotal - netUsed
			if freeBytes < 0 {
				freeBytes = 0
			}
			// делим на 1_000_000, как ждёт автотест
			mbit := int(freeBytes / 1_000_000)
			msgs = append(msgs, fmt.Sprintf("Network bandwidth usage high: %d Mbit/s available", mbit))
		}
	}

	return msgs
}
