package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"limiter/service"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/assert/v2"
)

func TestLimiter(t *testing.T) {
	type testCase struct {
		ttl, threshold, updateTime uint
	}
	testCases := []testCase{
		{1000, 10, 300},   // Rate limit of 10 requests per second, updateTime of 300ms
		{1000, 1000, 500}, // Rate limit of 1000 requests per second, updateTime of 500ms
		{500, 200, 5000},  // Rate limit of 200 requests per 500ms, updateTime of 5000ms
	}

	for _, tc := range testCases {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		limiter := service.GetLimiter(tc.threshold, tc.ttl, tc.updateTime)
		router := setupRouter(limiter)
		srv := &http.Server{
			Addr:    ":8080",
			Handler: router,
		}
		go func() {
			// Wait for the context to be cancelled
			<-ctx.Done()
			// t.Log("ctx.Done")
			log.Println("ctx.Done")

			// Shutdown the server gracefully
			srv.Shutdown(ctx)
		}()

		// Example for rate limit of 10 requests per second, updateTime of 300ms
		// Test 1: Updating the counter with no rate limit
		// expect to responses body {"count":1} - {"count":10}
		for i := int32(0); i < int32(tc.threshold); i++ {
			t.Log("Test 1")
			testPost(t, router, 1, i+1, false)

		}
		// Test 2: Rate limit (more then threshold requests in ttl)
		// expect to responses body {"delayed":1} - {"delayed":5}
		for i := int32(0); i < int32(tc.threshold/2); i++ {
			t.Log("Test 2")
			testPost(t, router, 1, i+1, true)
		}
		// Test 3: Wait for the TTL to expire and then update the counter from cache (+5)
		// expect to response body {"count":16}
		time.Sleep(time.Duration(tc.ttl+100) * time.Millisecond)
		t.Log("Test 3")
		testPost(t, router, 1, int32(tc.threshold+tc.threshold/2+1), false)

		// Test 4: Wait for the updateTime to expire and then update the counter
		// expect to response body {"count":17}
		time.Sleep(time.Duration(tc.updateTime+100) * time.Millisecond)
		t.Log("Test 4")
		testPost(t, router, 1, int32(tc.threshold+tc.threshold/2+2), false)

		// Test 5: GET test
		// expect to response body {"count":17}
		t.Log("Test 5")
		testGet(t, router, int32(tc.threshold+tc.threshold/2+2))

		// Test 6: Send invalid data in the POST request
		// expect to response body {"count":17} - without changes
		t.Log("Test 6")
		jsonObj := map[string]int{
			"invalid_key": 100,
		}
		body, _ := json.Marshal(jsonObj)
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/counter", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		respBody := w.Body.Bytes()
		var response IncrementResponse
		err := json.Unmarshal(respBody, &response)
		if err != nil {
			fmt.Println("error:", err)
		}
		// assert http response
		assert.Equal(t, http.StatusBadRequest, 400)
	}

}

func TestEdge(t *testing.T) {
	// Test Edge case with Rate limit of 1000 requests per second and updateTime of 400ms
	// in this case there is a time that the db updated (updateTimer expired) but the requests is still blocked(TTLTimer running)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ttl, threshold, updateTime := uint(1000), uint(100), uint(400)
	limiter := service.GetLimiter(threshold, ttl, updateTime)
	router := setupRouter(limiter)
	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}
	go func() {
		// Wait for the context to be cancelled
		<-ctx.Done()
		log.Println("ctx.Done")

		// Shutdown the server gracefully
		srv.Shutdown(ctx)
	}()

	// startServer(router)
	// Updating the counter with no rate limit
	// expect to responses body {"count":1} - {"count":100}
	for i := int32(0); i < int32(threshold); i++ {
		testPost(t, router, 1, i+1, false)
		t.Log("edgeTest 1")

	}
	// Rate limit (more then threshold requests in ttl)
	// expect to responses body {"delayed":1} - {"delayed":50}
	for i := int32(0); i < int32(threshold/2); i++ {
		testPost(t, router, 1, i+1, true)
		t.Log("edgeTest 2")
	}

	// Edge test: Wait for the updateTime to expire and then GET the counter
	// expect to response body {"count":150}
	time.Sleep(time.Duration(updateTime+100) * time.Millisecond)
	testGet(t, router, int32(threshold+threshold/2))
	t.Log("edgeTest 3")

	// Trying update the counter - TTL timer Still running
	// expect to responses body {"delayed":1}
	testPost(t, router, 1, int32(1), true)
	t.Log("edgeTest 4")

}

func testPost(t *testing.T, router *gin.Engine, inc int32, expected int32, blocked bool) {
	//prepare request body
	jsonObj := IncrementRequest{
		Inc: inc,
	}
	body, err := json.Marshal(jsonObj)
	if err != nil {
		t.Error(err, "cannot marshal json")
		return
	}

	w := httptest.NewRecorder()
	req, err := http.NewRequest("POST", "/counter", bytes.NewBuffer(body))
	if err != nil {
		t.Error(err, "cannot create request")
		return
	}
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	//Check for expected response
	respBody := w.Body

	var resp IncrementResponse
	err = json.Unmarshal(respBody.Bytes(), &resp)
	if err != nil {
		fmt.Println("error:", err)
	}
	assert.Equal(t, resp.Count, expected)
	assert.Equal(t, resp.IsBlocked, blocked)

}

func testGet(t *testing.T, router *gin.Engine, expected int32) {

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/counter", nil)
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)
	//Check for expected response
	respBody := w.Body.Bytes()

	var response IncrementResponse
	err := json.Unmarshal(respBody, &response)
	if err != nil {
		fmt.Println("error:", err)
	}

	assert.Equal(t, response.Count, expected)

}
