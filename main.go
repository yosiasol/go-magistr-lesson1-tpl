package main

import (
	"bufio"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	statsURL       = "http://srv.msk01.gigacorp.local/_stats"
	requestTimeout = 1 * time.Second
	pollInterval   = 1 * time.Second
	maxErrorCount  = 3
)

func main() {
	client := &http.Client{
		Timeout: requestTimeout,
	}

	errorCount := 0

	for {
		ok, err := pollServer(client)
		if !ok || err != nil {
			errorCount++
		} else {
			// при успешном запросе сбрасываем счётчик ошибок
			errorCount = 0
		}

		if errorCount >= maxErrorCount {
			fmt.Println("Unable to fetch server statistic")
		}

		time.Sleep(pollInterval)
	}
}

// pollServer делает один запрос к серверу, парсит ответ и при необходимости выводит алерты.
func pollServer(client *http.Client) (bool, error) {
	req, err := http.NewRequest(http.MethodGet, statsURL, nil)
	if err != nil {
		return false, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// статус не 200 – считаем, что статистика недоступна
		return false, nil
	}

	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		// пустой ответ
		return false, nil
	}
	line := strings.TrimSpace(scanner.Text())
	if line == "" {
		return false, nil
	}

	parts := strings.Split(line, ",")
	if len(parts) != 7 {
		// формат данных не соответствует ожидаемому
		return false, nil
	}

	parse := func(s string) (float64, bool) {
		val, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if err != nil {
			return 0, false
		}
		return val, true
	}

	loadAvg, ok := parse(parts[0])
	if !ok {
		return false, nil
	}

	memTotal, ok := parse(parts[1])
	if !ok {
		return false, nil
	}
	memUsed, ok := parse(parts[2])
	if !ok {
		return false, nil
	}

	diskTotal, ok := parse(parts[3])
	if !ok {
		return false, nil
	}
	diskUsed, ok := parse(parts[4])
	if !ok {
		return false, nil
	}

	netBandwidth, ok := parse(parts[5])
	if !ok {
		return false, nil
	}
	netUsage, ok := parse(parts[6])
	if !ok {
		return false, nil
	}

	// 1. Load Average
	if loadAvg > 30 {
		// выводим исходное текстовое значение, как в ответе
		fmt.Printf("Load Average is too high: %s\n", strings.TrimSpace(parts[0]))
	}

	// 2. Память: > 80% от общего объёма
	if memTotal > 0 {
		memPercent := memUsed * 100 / memTotal
		if memPercent > 80 {
			fmt.Printf("Memory usage too high: %.0f%%\n", memPercent)
		}
	}

	// 3. Диск: > 90% занятого пространства
	if diskTotal > 0 {
		diskPercent := diskUsed * 100 / diskTotal
		if diskPercent > 90 {
			freeBytes := diskTotal - diskUsed
			if freeBytes < 0 {
				freeBytes = 0
			}
			// байты -> мегабайты, целочисленное деление (отбрасываем дробную часть)
			freeMB := int64(freeBytes / (1024 * 1024))
			fmt.Printf("Free disk space is too low: %d Mb left\n", freeMB)
		}
	}

	// 4. Сеть: > 90% занятой полосы
	if netBandwidth > 0 {
		netPercent := netUsage * 100 / netBandwidth
		if netPercent > 90 {
			freeBytesPerSec := netBandwidth - netUsage
			if freeBytesPerSec < 0 {
				freeBytesPerSec = 0
			}
			// свободная полоса в "мегабайтах" в секунду (делим только на 1_000_000)
			freeM := int64(freeBytesPerSec / 1_000_000)
			fmt.Printf("Network bandwidth usage high: %d Mbit/s available\n", freeM)
		}
	}

	return true, nil
}
