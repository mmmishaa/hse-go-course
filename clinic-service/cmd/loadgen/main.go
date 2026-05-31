package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

var baseURL = "http://app:8080"

type RegisterReq struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Phone     string `json:"phone"`
	Email     string `json:"email"`
	Password  string `json:"password"`
}

type LoginReq struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginResp struct {
	Token string `json:"token"`
}

type CreateApptReq struct {
	Doctor           string `json:"doctor"`
	DoctorSpeciality string `json:"doctor_speciality"`
	DoctorOffice     string `json:"doctor_office"`
	Date             string `json:"date"`
}

type Stats struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TotalDuration   int64
}

func main() {
	concurrency := 50
	totalRequests := 300

	var stats Stats
	var wg sync.WaitGroup
	sem := make(chan struct{}, concurrency)

	start := time.Now()
	log.Printf("Starting load test: %d requests, concurrency %d", totalRequests, concurrency)

	// Получаем список врачей
	doctors, err := getDoctors()
	if err != nil {
		log.Fatalf("Failed to get doctors: %v", err)
	}
	if len(doctors) == 0 {
		log.Fatal("No doctors available")
	}

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer wg.Done()
			defer func() { <-sem }()

			reqStart := time.Now()
			atomic.AddInt64(&stats.TotalRequests, 1)

			success := performUserFlow(idx, doctors)

			duration := time.Since(reqStart).Milliseconds()
			atomic.AddInt64(&stats.TotalDuration, duration)

			if success {
				atomic.AddInt64(&stats.SuccessRequests, 1)
			} else {
				atomic.AddInt64(&stats.FailedRequests, 1)
			}
		}(i)
	}

	wg.Wait()
	totalDuration := time.Since(start)

	fmt.Printf("\n=== Load Test Results ===\n")
	fmt.Printf("Total requests: %d\n", stats.TotalRequests)
	fmt.Printf("Successful: %d (%.1f%%)\n", stats.SuccessRequests,
		float64(stats.SuccessRequests)/float64(stats.TotalRequests)*100)
	fmt.Printf("Failed: %d (%.1f%%)\n", stats.FailedRequests,
		float64(stats.FailedRequests)/float64(stats.TotalRequests)*100)
	fmt.Printf("Total duration: %v\n", totalDuration)
	fmt.Printf("Average request time: %.2f ms\n",
		float64(stats.TotalDuration)/float64(stats.TotalRequests))
	fmt.Printf("Requests per second: %.2f\n",
		float64(stats.TotalRequests)/totalDuration.Seconds())
}

func performUserFlow(idx int, doctors []map[string]interface{}) bool {
	email := fmt.Sprintf("user%d@test.com", idx)
	password := "password123"
	firstName := fmt.Sprintf("User%d", idx)
	lastName := fmt.Sprintf("Test%d", idx)
	phone := fmt.Sprintf("+7999%07d", idx)

	// 1. Регистрация
	reg := RegisterReq{
		FirstName: firstName,
		LastName:  lastName,
		Phone:     phone,
		Email:     email,
		Password:  password,
	}
	body, _ := json.Marshal(reg)
	resp, err := http.Post(baseURL+"/auth/register", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("[idx=%d] register error: %v", idx, err)
		return false
	}
	io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode > 299 && resp.StatusCode != 400 {
		log.Printf("[idx=%d] register failed: status %d", idx, resp.StatusCode)
		return false
	}

	// 2. Логин
	login := LoginReq{Email: email, Password: password}
	body, _ = json.Marshal(login)
	resp, err = http.Post(baseURL+"/auth/login", "application/json", bytes.NewReader(body))
	if err != nil || resp.StatusCode != 200 {
		if resp != nil {
			resp.Body.Close()
		}
		log.Printf("[idx=%d] login failed", idx)
		return false
	}
	var loginResp LoginResp
	json.NewDecoder(resp.Body).Decode(&loginResp)
	resp.Body.Close()

	// 3. Генерация ВАЛИДНОЙ даты для записи
	futureDate := generateValidDate()

	// 4. Создание записи (берём случайного врача)
	doctor := doctors[rand.Intn(len(doctors))]
	doctorName, _ := doctor["name"].(string)
	doctorSpeciality, _ := doctor["speciality"].(string)
	doctorOffice, _ := doctor["office"].(string)

	appt := CreateApptReq{
		Doctor:           doctorName,
		DoctorSpeciality: doctorSpeciality,
		DoctorOffice:     doctorOffice,
		Date:             futureDate.Format(time.RFC3339),
	}
	apptBody, _ := json.Marshal(appt)
	req, _ := http.NewRequest("POST", baseURL+"/appointments/create", bytes.NewReader(apptBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+loginResp.Token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err = client.Do(req)
	if err != nil {
		log.Printf("[idx=%d] create appointment error: %v", idx, err)
		return false
	}
	bodyBytes, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode > 299 {
		log.Printf("[idx=%d] create appointment failed: status %d, body: %s", idx, resp.StatusCode, string(bodyBytes))
		return false
	}

	// 5. Получение списка записей (опционально)
	req2, _ := http.NewRequest("GET", baseURL+"/appointments/list", nil)
	req2.Header.Set("Authorization", "Bearer "+loginResp.Token)
	resp2, err := client.Do(req2)
	if err == nil {
		io.ReadAll(resp2.Body)
		resp2.Body.Close()
	}

	return true
}

// generateValidDate создает дату, которая точно пройдёт все валидации:
// - не выходной (пн-пт)
// - рабочее время (9:00-18:00, кроме 13:00 обеда)
// - только целые часы (минуты и секунды = 0)
// - не менее чем через 1 час от текущего момента
// - не более чем через 30 дней
func generateValidDate() time.Time {
	// Случайное смещение от 1 до 20 дней вперёд
	daysOffset := 1 + rand.Intn(20)
	targetDay := time.Now().AddDate(0, 0, daysOffset)

	// Если попался выходной — сдвигаем на понедельник
	if targetDay.Weekday() == time.Saturday {
		targetDay = targetDay.AddDate(0, 0, 2)
	} else if targetDay.Weekday() == time.Sunday {
		targetDay = targetDay.AddDate(0, 0, 1)
	}

	// Рабочие часы (без обеда 13:00)
	hours := []int{9, 10, 11, 12, 14, 15, 16, 17}
	hour := hours[rand.Intn(len(hours))]

	// Формируем дату с нулевыми минутами и секундами
	return time.Date(
		targetDay.Year(),
		targetDay.Month(),
		targetDay.Day(),
		hour, 0, 0, 0,
		time.UTC,
	)
}

func getDoctors() ([]map[string]interface{}, error) {
	resp, err := http.Get(baseURL + "/doctors")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var doctors []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&doctors); err != nil {
		return nil, fmt.Errorf("decode error: %w", err)
	}

	if len(doctors) == 0 {
		return nil, fmt.Errorf("no doctors returned")
	}

	return doctors, nil
}