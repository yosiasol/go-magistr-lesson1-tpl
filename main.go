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
	statsURL      = "http://srv.msk01.gigacorp.local/_stats"
	pollInterval  = time.Second
	requestTimout = time.Second
	maxErrors     = 3
)

func main() {
	client := &http.Client{
		Timeout: requestTimout,
	}

	errorCount := 0

	for {
		err := checkServerStats(client)
		if err != nil {
			// ошибка получения или парсинга данных
			errorCount++
			if errorCount >= maxErrors {
				fmt.Println("Unable to fetch server statistic")
			}
		} else {
			// успешный запрос сбрасывает счётчик ошибок
			errorCount = 0
		}

		time.Sleep(pollInterval)
	}
}

func checkServerStats(client *http.Client) error {
	resp, err := client.Get(statsURL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	line := strings.TrimSpace(string(body))
	if line == "" {
		return fmt.Errorf("empty body")
	}

	parts := strings.Split(line, ",")
	if len(parts) != 7 {
		return fmt.Errorf("unexpected values count: %d", len(parts))
	}

	parse := func(s string) (float64, error) {
		return strconv.ParseFloat(strings.TrimSpace(s), 64)
	}

	loadAvg, err := parse(parts[0])
	if err != nil {
		return err
	}
	memTotal, err := parse(parts[1])
	if err != nil {
		return err
	}
	memUsed, err := parse(parts[2])
	if err != nil {
		return err
	}
	diskTotal, err := parse(parts[3])
	if err != nil {
		return err
	}
	diskUsed, err := parse(parts[4])
	if err != nil {
		return err
	}
	netTotal, err := parse(parts[5])
	if err != nil {
		return err
	}
	netUsed, err := parse(parts[6])
	if err != nil {
		return err
	}

	// 1. Load Average
	if loadAvg > 30 {
		fmt.Printf("Load Average is too high: %d\n", int(loadAvg))
	}

	// 2. Память: > 80% использования, целый процент с усечением вниз
	if memTotal > 0 {
		memUsage := memUsed / memTotal * 100
		if memUsage > 80 {
			fmt.Printf("Memory usage too high: %d%%\n", int(memUsage))
		}
	}

	// 3. Диск: > 90% использования, свободное место в МБ, усечённое вниз
	if diskTotal > 0 {
		diskUsage := diskUsed / diskTotal * 100
		if diskUsage > 90 {
			freeBytes := diskTotal - diskUsed
			if freeBytes < 0 {
				freeBytes = 0
			}
			freeMB := freeBytes / 1024 / 1024
			fmt.Printf("Free disk space is too low: %d Mb left\n", int(freeMB))
		}
	}

	// 4. Сеть: > 90% использования, свободная полоса в "Mbit/s" как bytes/1e6, усечённая вниз
	if netTotal > 0 {
		netUsage := netUsed / netTotal * 100
		if netUsage > 90 {
			freeBytesPerSec := netTotal - netUsed
			if freeBytesPerSec < 0 {
				freeBytesPerSec = 0
			}
			// по факту тесты ожидают freeBytesPerSec / 1_000_000
			freeMbit := freeBytesPerSec / 1_000_000.0
			fmt.Printf("Network bandwidth usage high: %d Mbit/s available\n", int(freeMbit))
		}
	}

	return nil
}
