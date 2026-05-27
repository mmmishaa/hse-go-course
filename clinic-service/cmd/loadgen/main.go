package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

var baseURL = "http://app:8080"

func main() {
	var wg sync.WaitGroup
	start := time.Now()
	concurrency := 50
	total := 300
	sem := make(chan struct{}, concurrency)
	var success, fail int
	var mu sync.Mutex

	for i := 0; i < total; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			email := fmt.Sprintf("user%d@test.com", idx)
			pass := "password123"

			reg := map[string]string{"email": email, "password": pass}
			body, _ := json.Marshal(reg)
			resp, err := http.Post(baseURL+"/auth/register", "application/json", bytes.NewReader(body))
			if err != nil || resp.StatusCode > 299 {
				mu.Lock()
				fail++
				mu.Unlock()
				return
			}
			io.ReadAll(resp.Body)
			resp.Body.Close()

			resp, err = http.Post(baseURL+"/auth/login", "application/json", bytes.NewReader(body))
			if err != nil {
				mu.Lock()
				fail++
				mu.Unlock()
				return
			}
			var tokenResp struct {
				Token string `json:"token"`
			}
			json.NewDecoder(resp.Body).Decode(&tokenResp)
			resp.Body.Close()

			doctors := []string{"Иванов И.И.", "Петров П.П.", "Сидоров С.С."}
			apt := map[string]interface{}{
				"doctor": doctors[rand.Intn(len(doctors))],
				"date":   time.Now().Add(time.Hour * time.Duration(rand.Intn(48))).Format(time.RFC3339),
			}
			aptBody, _ := json.Marshal(apt)
			req, _ := http.NewRequest("POST", baseURL+"/appointments/create", bytes.NewReader(aptBody))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+tokenResp.Token)
			client := &http.Client{Timeout: 10 * time.Second}
			resp, err = client.Do(req)
			if err != nil || resp.StatusCode > 299 {
				mu.Lock()
				fail++
				mu.Unlock()
				return
			}
			io.ReadAll(resp.Body)
			resp.Body.Close()

			req2, _ := http.NewRequest("GET", baseURL+"/appointments/list", nil)
			req2.Header.Set("Authorization", "Bearer "+tokenResp.Token)
			resp2, _ := client.Do(req2)
			io.ReadAll(resp2.Body)
			resp2.Body.Close()

			mu.Lock()
			success++
			mu.Unlock()
		}(i)
	}
	wg.Wait()
	fmt.Printf("Load test finished in %v\nSuccess: %d, Fail: %d\n", time.Since(start), success, fail)
}