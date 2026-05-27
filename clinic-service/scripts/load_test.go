// scripts/load_test.go
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

func main() {
	var wg sync.WaitGroup
	start := time.Now()
	concurrency := 50
	total := 500
	sem := make(chan struct{}, concurrency)
	var success, fail int
	var mu sync.Mutex

	for i := 0; i < total; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			
			// Register
			reg, _ := json.Marshal(map[string]string{"email": fmt.Sprintf("user%d@test.com", i), "password": "123"})
			resp, err := http.Post("http://server:8080/register", "application/json", bytes.NewReader(reg))
			if err != nil || resp.StatusCode > 299 {
				mu.Lock(); fail++; mu.Unlock(); return
			}
			defer resp.Body.Close()

			// Login
			resp, _ = http.Post("http://server:8080/login", "application/json", bytes.NewReader(reg))
			var tokenResp struct{ Token string }
			json.NewDecoder(resp.Body).Decode(&tokenResp)
			defer resp.Body.Close()

			// Create Appointment
			apt, _ := json.Marshal(map[string]interface{}{"doctor": "Ivanov", "date": "2024-12-01T10:00:00Z"})
			req, _ := http.NewRequest("POST", "http://server:8080/appointments", bytes.NewReader(apt))
			req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
			client := &http.Client{Timeout: 5 * time.Second}
			resp, err = client.Do(req)
			if err != nil || resp.StatusCode > 299 {
				mu.Lock(); fail++; mu.Unlock(); return
			}
			io.ReadAll(resp.Body)
			resp.Body.Close()

			// Get Appointments
			req2, _ := http.NewRequest("GET", "http://server:8080/appointments", nil)
			req2.Header.Set("Authorization", "Bearer "+tokenResp.Token)
			resp2, _ := client.Do(req2)
			io.ReadAll(resp2.Body)
			resp2.Body.Close()

			mu.Lock(); success++; mu.Unlock()
		}()
	}
	wg.Wait()
	fmt.Printf("Load test finished in %v\nSuccess: %d, Fail: %d\n", time.Since(start), success, fail)
}