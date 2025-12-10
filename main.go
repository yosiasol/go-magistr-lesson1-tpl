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
	statsURL            = "http://srv.msk01.gigacorp.local/_stats"
	loadAvgThreshold    = 30.0  // Load Average > 30
	memUsageThreshold   = 80.0  // Память > 80% от доступной
	diskUsageThreshold  = 90.0  // Диск > 90% от доступного
	netUsageThreshold   = 90.0  // Сеть > 90% от полосы
	freeDiskDivisorMB   = 1024 * 1024 // байт в мегабайте
	freeNetworkDivisor  = 1_000_000   // байт в "десятичном" мегабите
	requestTimeout      = 5 * time.Second
	pollInterval        = 1 * time.Second
	minErrorsForMessage = 3
)

func main() {
	client := &http.Client{
		Timeout: requestTimeout,
	}

	errorCount := 0

	for {
		ok := pollOnce(client)
		if ok {
			// Успешный запрос сбрасывает счетчик ошибок
			errorCount = 0
		} else {
			// Ошибка получения/разбора статистики
			errorCount++
			if errorCount >= minErrorsForMessage {
				fmt.Println("Unable to fetch server statistic")
			}
		}

		time.Sleep(pollInterval)
	}
}

// pollOnce выполняет один запрос к серверу статистики.
// Возвращает true, если данные успешно получены и обработаны.
// Возвращает false при любой сетевой ошибке, неверном статусе или формате данных.
func pollOnce(client *http.Client) bool {
	resp, err := client.Get(statsURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false
	}

	line := strings.TrimSpace(string(body))
	if line == "" {
		return false
	}

	parts := strings.Split(line, ",")
	if len(parts) != 7 {
		// Формат не соответствует ожидаемому
		return false
	}

	parse := func(s string) (float64, bool) {
		v, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return 0, false
		}
		return v, true
	}

	loadAvg, ok := parse(parts[0])
	if !ok {
		return false
	}

	totalMem, ok := parse(parts[1])
	if !ok {
		return false
	}

	usedMem, ok := parse(parts[2])
	if !ok {
		return false
	}

	totalDisk, ok := parse(parts[3])
	if !ok {
		return false
	}

	usedDisk, ok := parse(parts[4])
	if !ok {
		return false
	}

	netBandwidth, ok := parse(parts[5])
	if !ok {
		return false
	}

	netUsage, ok := parse(parts[6])
	if !ok {
		return false
	}

	// 1. Load Average
	if loadAvg > loadAvgThreshold {
		// Выводим целое значение, как в условии
		fmt.Printf("Load Average is too high: %.0f\n", loadAvg)
	}

	// 2. Память
	if totalMem > 0 {
		memUsagePercent := usedMem * 100 / totalMem
		if memUsagePercent > memUsageThreshold {
			fmt.Printf("Memory usage too high: %d%%\n", int(memUsagePercent))
		}
	}

	// 3. Диск
	if totalDisk > 0 {
		diskUsagePercent := usedDisk * 100 / totalDisk
		if diskUsagePercent > diskUsageThreshold {
			freeBytes := totalDisk - usedDisk
			if freeBytes < 0 {
				freeBytes = 0
			}
			freeMB := int(freeBytes / freeDiskDivisorMB)
			fmt.Printf("Free disk space is too low: %d Mb left\n", freeMB)
		}
	}

	// 4. Сеть
	if netBandwidth > 0 {
		netUsagePercent := netUsage * 100 / netBandwidth
		if netUsagePercent > netUsageThreshold {
			freeBytesPerSec := netBandwidth - netUsage
			if freeBytesPerSec < 0 {
				freeBytesPerSec = 0
			}
			// Переводим байты в секунду в мегабиты в секунду:
			// байты * 8 / 1_000_000
			freeMbitPerSec := int(freeBytesPerSec * 8 / freeNetworkDivisor)
			fmt.Printf("Network bandwidth usage high: %d Mbit/s available\n", freeMbitPerSec)
		}
	}

	return true
}
