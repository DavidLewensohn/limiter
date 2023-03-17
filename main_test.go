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

type CounterResponse struct {
	Count int32 `json:"count"`
}
type DelayedResponse struct {
	Count int32 `json:"delayed"`
}

func Test(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(3)
	go func() {
		defer wg.Done()
		// Rate limit of 10 requests per second, updateTime of 300ms
		testLimiter(t, 1000, 10, 300)
	}()
	go func() {
		defer wg.Done()
		// High rate limit of 4000 requests per second, updateTime of 500ms
		testLimiter(t, 1000, 4000, 500)
	}()
	go func() {
		defer wg.Done()
		//Rate limit of 200 requests per  half second, updateTime of 5000ms
		testLimiter(t, 500, 200, 5000)
	}()
	wg.Wait()
	// Test Edge case with updateTimer longer than TTLTimer
	testEdge(t)
}

func testLimiter(t *testing.T, ttl uint, threshold uint, updateTime uint) {

	limiter := service.GetLimiter(threshold, ttl, updateTime)
	server := newServer(limiter)
	// Test 1: Updating the counter with no rate limit
	// expect to responses body {"count":1} - {"count":10}
	for i := int32(0); i < int32(threshold); i++ {
		testPost(t, server, 1, i+1, false)

	}
	// Test 2: Rate limit (more then threshold requests in ttl)
	// expect to responses body {"delayed":1} - {"delayed":5}
	for i := int32(0); i < int32(threshold/2); i++ {
		testPost(t, server, 1, i+1, true)
	}
	// Test 3: Wait for the TTL to expire and then update the counter from cache (+5)
	// expect to response body {"count":16}
	time.Sleep(time.Duration(ttl+100) * time.Millisecond)
	testPost(t, server, 1, int32(threshold+threshold/2+1), false)

	// Test 4: Wait for the updateTime to expire and then update the counter
	// expect to response body {"count":17}
	time.Sleep(time.Duration(updateTime+100) * time.Millisecond)
	testPost(t, server, 1, int32(threshold+threshold/2+2), false)

	// Test 5: GET test
	// expect to response body {"count":17}
	testGet(t, server, int32(threshold+threshold/2+2))

	// Test 6: Send invalid data in the POST request
	// expect to response body {"count":17} - without changes
	jsonObj := map[string]int{
		"invalid_key": 100,
	}
	body, _ := json.Marshal(jsonObj)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/counter", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	server.ServeHTTP(w, req)
	respBody := w.Body.Bytes()
	var counter CounterResponse
	err := json.Unmarshal(respBody, &counter)
	if err != nil {
		fmt.Println("error:", err)
	}

	assert.Equal(t, counter.Count, int32(threshold+threshold/2+2))

}

func testEdge(t *testing.T) {
	// Test Edge case with Rate limit of 1000 requests per second and updateTime of 400ms
	// in this case there is a time that the db updated (updateTimer expired) but the requests is still blocked(TTLTimer running)

	ttl, threshold, updateTime := uint(1000), uint(100), uint(400)
	limiter := service.GetLimiter(threshold, ttl, updateTime)
	server := newServer(limiter)
	// Updating the counter with no rate limit
	// expect to responses body {"count":1} - {"count":100}
	for i := int32(0); i < int32(threshold); i++ {
		testPost(t, server, 1, i+1, false)

	}
	// Rate limit (more then threshold requests in ttl)
	// expect to responses body {"delayed":1} - {"delayed":50}
	for i := int32(0); i < int32(threshold/2); i++ {
		testPost(t, server, 1, i+1, true)
	}

	// Edge test: Wait for the updateTime to expire and then GET the counter
	// expect to response body {"count":150}
	time.Sleep(time.Duration(updateTime+100) * time.Millisecond)
	testGet(t, server, int32(threshold+threshold/2))

	// Trying update the counter - TTL timer Still running
	// expect to responses body {"delayed":1}
	testPost(t, server, 1, int32(1), true)

}

func testPost(t *testing.T, router *gin.Engine, inc int32, expected int32, blocked bool) {
	fmt.Println("inc: ", inc)

	//prepare request body
	jsonObj := IncrementRequest{
		Inc: inc,
	}
	body, err := json.Marshal(jsonObj)
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

	if !blocked {
		var counter CounterResponse
		err := json.Unmarshal(respBody.Bytes(), &counter)
		if err != nil {
			fmt.Println("error:", err)
		}
		assert.Equal(t, counter.Count, expected)
	} else {
		var delayed DelayedResponse
		err := json.Unmarshal(respBody.Bytes(), &delayed)
		if err != nil {
			fmt.Println("error:", err)
		}

		assert.Equal(t, delayed.Count, expected)
	}

}

func testGet(t *testing.T, router *gin.Engine, expected int32) {

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/counter", nil)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	//Check for expected response
	respBody := w.Body.Bytes()

	var counter CounterResponse
	err := json.Unmarshal(respBody, &counter)
	if err != nil {
		fmt.Println("error:", err)
	}
	assert.Equal(t, counter.Count, expected)

}
