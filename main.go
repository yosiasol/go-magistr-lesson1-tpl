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
			// Ошибка получения или парсинга данных
			errorCount++
			if errorCount >= maxErrors {
				fmt.Println("Unable to fetch server statistic")
			}
		} else {
			// Успешный запрос сбрасывает счётчик ошибок
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

	values := strings.Split(strings.TrimSpace(string(body)), ",")
	if len(values) != 7 {
		return fmt.Errorf("unexpected values count: %d", len(values))
	}

	parse := func(s string) (float64, error) {
		return strconv.ParseFloat(strings.TrimSpace(s), 64)
	}

	loadAvg, err := parse(values[0])
	if err != nil {
		return err
	}
	memTotal, err := parse(values[1])
	if err != nil {
		return err
	}
	memUsed, err := parse(values[2])
	if err != nil {
		return err
	}
	diskTotal, err := parse(values[3])
	if err != nil {
		return err
	}
	diskUsed, err := parse(values[4])
	if err != nil {
		return err
	}
	netTotal, err := parse(values[5])
	if err != nil {
		return err
	}
	netUsed, err := parse(values[6])
	if err != nil {
		return err
	}

	// 1. Load Average
	if loadAvg > 30 {
		fmt.Printf("Load Average is too high: %d\n", int(loadAvg))
	}

	// 2. Память: если > 80% (строго), печатаем целый процент, усечённый вниз
	if memTotal > 0 {
		memUsage := memUsed / memTotal * 100
		if memUsage > 80 {
			fmt.Printf("Memory usage too high: %d%%\n", int(memUsage))
		}
	}

	// 3. Диск: если занято > 90%, печатаем свободное место в мегабайтах (floor)
	if diskTotal > 0 {
		diskUsedPct := diskUsed / diskTotal * 100
		if diskUsedPct > 90 {
			freeBytes := diskTotal - diskUsed
			if freeBytes < 0 {
				freeBytes = 0
			}
			freeMB := freeBytes / 1024 / 1024
			fmt.Printf("Free disk space is too low: %d Mb left\n", int(freeMB))
		}
	}

	// 4. Сеть: если занято > 90%, печатаем свободную полосу в мегабитах/сек (floor)
	if netTotal > 0 {
		netUsedPct := netUsed / netTotal * 100
		if netUsedPct > 90 {
			freeNet := netTotal - netUsed
			if freeNet < 0 {
				freeNet = 0
			}
			freeMbit := freeNet / 1_000_000.0
			fmt.Printf("Network bandwidth usage high: %d Mbit/s available\n", int(freeMbit))
		}
	}

	return nil
}
