package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"limiter/service"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/assert/v2"
)

type Counter struct {
	Count int32 `json:"count"`
}
type Delayed struct {
	Count int32 `json:"delayed"`
}

func Test(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		testLimiter(t, 1000, 10, 300)
	}()
	go func() {
		defer wg.Done()
		testLimiter(t, 400, 10, 800)
	}()
	wg.Wait()
}

func testLimiter(t *testing.T, ttl uint, threshold uint, updateTime uint) {

	limiter := service.GetLimiter(threshold, ttl, updateTime)
	server := newServer(limiter)
	// Test 1: Updating the counter with no rate limit
	// expect to responses body {"count":1} - {"count":10}
	for i := int32(0); i < 10; i++ {
		testPost(t, server, 1, i+1, false)

	}
	// Test 2: Rate limit (more then threshold requests in ttl)
	// expect to responses body {"delayed":1} - {"delayed":5}
	for i := int32(0); i < 5; i++ {
		testPost(t, server, 1, i+1, true)
	}
	// Test 3: Wait for the TTL to expire and then update the counter from cache (+5)
	// expect to response body {"count":16}
	time.Sleep(time.Duration(ttl+100) * time.Millisecond)
	testPost(t, server, 1, 16, false)

	// Test 4: Wait for the updateTime to expire and then update the counter
	// expect to response body {"count":17}
	time.Sleep(time.Duration(updateTime+100) * time.Millisecond)
	testPost(t, server, 1, 17, false)

	// Test 5: Send invalid data in the POST request
	//  expect to response body {"count":17} - without changes
	jsonObj := map[string]int{
		"invalid_key": 100,
	}
	body, _ := json.Marshal(jsonObj)
	// fmt.Println("body: ", string(body))
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/counter", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(w, req)
	respBody := w.Body.Bytes()
	// fmt.Println("respBody: ", respBody)
	var counter Counter
	err := json.Unmarshal(respBody, &counter)
	if err != nil {
		fmt.Println("error:", err)
	}

	assert.Equal(t, counter.Count, int32(17))

}

func testPost(t *testing.T, router *gin.Engine, inc int32, expected int32, blocked bool) {
	fmt.Println("inc: ", inc)

	//prepare request body
	jsonObj := IncrementRequest{
		Inc: inc,
	}
	body, err := json.Marshal(jsonObj)
	// fmt.Println("jsonObj: ", jsonObj)
	if err != nil {
		fmt.Println(err)
		return
	}

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/counter", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	//Check for expected response
	respBody := w.Body
	// fmt.Println("respBody: ", respBody)

	if !blocked {
		var counter Counter
		err := json.Unmarshal(respBody.Bytes(), &counter)
		if err != nil {
			fmt.Println("error:", err)
		}
		// fmt.Println("counter: ", counter)
		assert.Equal(t, counter.Count, expected)
	} else {
		var delayed Delayed
		err := json.Unmarshal(respBody.Bytes(), &delayed)
		if err != nil {
			fmt.Println("error:", err)
		}

		assert.Equal(t, delayed.Count, expected)
	}

}
