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
			// при успешном запросе сбрасываем счетчик ошибок
			errorCount = 0
		}

		if errorCount >= maxErrorCount {
			fmt.Println("Unable to fetch server statistic")
		}

		time.Sleep(pollInterval)
	}
}

// pollServer делает один запрос к серверу, парсит ответ и при необходимости выводит алерты.
// Возвращает:
//
//	ok == true  если данные успешно получены и распарсены;
//	ok == false если статус не 200 или формат данных неверный;
//	err != nil  если была сетевая или иная техническая ошибка.
func pollServer(client *http.Client) (ok bool, err error) {
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
		return false, nil
	}

	scanner := bufio.NewScanner(resp.Body)
	if !scanner.Scan() {
		return false, nil
	}
	line := strings.TrimSpace(scanner.Text())
	if line == "" {
		return false, nil
	}

	parts := strings.Split(line, ",")
	if len(parts) != 7 {
		return false, nil
	}

	// парсим числа
	loadAvg, ok := mustParseFloat(parts[0])
	if !ok {
		return false, nil
	}

	memTotal, ok := mustParseFloat(parts[1])
	if !ok {
		return false, nil
	}
	memUsed, ok := mustParseFloat(parts[2])
	if !ok {
		return false, nil
	}

	diskTotal, ok := mustParseFloat(parts[3])
	if !ok {
		return false, nil
	}
	diskUsed, ok := mustParseFloat(parts[4])
	if !ok {
		return false, nil
	}

	netBandwidth, ok := mustParseFloat(parts[5])
	if !ok {
		return false, nil
	}
	netUsage, ok := mustParseFloat(parts[6])
	if !ok {
		return false, nil
	}

	// 1. Load Average
	if loadAvg > 30 {
		// печатаем исходное текстовое значение, как пришло с сервера
		fmt.Printf("Load Average is too high: %s\n", strings.TrimSpace(parts[0]))
	}

	// 2. Память
	if memTotal > 0 {
		memPercent := memUsed / memTotal * 100
		if memPercent > 80 {
			fmt.Printf("Memory usage too high: %.0f%%\n", memPercent)
		}
	}

	// 3. Диск
	if diskTotal > 0 {
		diskPercent := diskUsed / diskTotal * 100
		if diskPercent > 90 {
			freeBytes := diskTotal - diskUsed
			freeMB := freeBytes / (1024 * 1024)
			fmt.Printf("Free disk space is too low: %.0f Mb left\n", freeMB)
		}
	}

	// 4. Сеть
	if netBandwidth > 0 {
		netPercent := netUsage / netBandwidth * 100
		if netPercent > 90 {
			freeBytesPerSec := netBandwidth - netUsage
			// байты в секунду -> мегабиты в секунду
			freeMbitPerSec := freeBytesPerSec * 8 / (1024 * 1024)
			fmt.Printf("Network bandwidth usage high: %.0f Mbit/s available\n", freeMbitPerSec)
		}
	}

	return true, nil
}

// mustParseFloat парсит число в формате float64.
// Возвращает false при ошибке парсинга.
// заменили версию
func mustParseFloat(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}
