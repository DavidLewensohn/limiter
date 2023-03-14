package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"limiter/service"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/assert/v2"
)

func TestLimiter(t *testing.T) {
	ttl := uint(60000)
	threshold := uint(10)
	updateTime := uint(30000)

	limiter := service.GetLimiter(threshold, ttl, updateTime)
	server := createServer(limiter)
	go func() {
		for i := 0; i < int(threshold/2); i++ {
			testPost(t, server, 1, true)
		}
	}()
	go func() {
		for i := 0; i < int(threshold/2); i++ {
			testPost(t, server, 1, true)
		}
	}()
	time.Sleep(time.Second)

	for i := 0; i < 5; i++ {
		testPost(t, server, 1, false)
	}
	testGet(t, server, int32(threshold))

	time.Sleep(time.Duration((ttl) * 1000000))
	for i := 0; i < int(threshold); i++ {
		testPost(t, server, 1, true)
	}
	testGet(t, server, int32(threshold*2+5))
}

func testPost(t *testing.T, router *gin.Engine, inc int, ok bool) {
	//prepare request body
	jsonObj := map[string]int{
		"inc": inc,
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
	if ok {
		assert.Equal(t, http.StatusOK, w.Code)
	} else {
		assert.Equal(t, http.StatusBadRequest, w.Code)
	}

}
func testGet(t *testing.T, router *gin.Engine, expectedCounter int32) {
	body, _ := json.Marshal(map[string]int{})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/counter", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	router.ServeHTTP(w, req)

	var a service.Counter
	json.Unmarshal(w.Body.Bytes(), &a)
	assert.Equal(t, expectedCounter, a.Count)

}
