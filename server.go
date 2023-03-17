package main

import (
	"flag"
	"limiter/service"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
)

func main() {
	threshold := flag.Uint("threshold", 5, "Max requests in a time period")
	ttl := flag.Uint("ttl", 10000, "Time to live in milliseconds")
	updateTime := flag.Uint("update", 10000, "Time with no post request in milliseconds to update the DB")
	flag.Parse()

	limiter := service.GetLimiter(*threshold, *ttl, *updateTime)
	server := newServer(limiter)
	server.Static("/counter", "./public")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server.Run(":" + port)
}

func newServer(limiter *service.Limiter) *gin.Engine {
	server := gin.Default()
	server.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	})

	server.GET("/counter", func(c *gin.Context) {
		c.JSON(http.StatusOK, limiter.GetCounter())
	})
	server.POST("/counter", func(c *gin.Context) {
		handleCounterIncrement(c, limiter)
	})
	server.OPTIONS("/counter", func(c *gin.Context) {
		c.AbortWithStatus(http.StatusNoContent)
	})
	return server
}
func handleCounterIncrement(c *gin.Context, limiter *service.Limiter) {
	var incrementReq IncrementRequest
	if err := c.ShouldBindJSON(&incrementReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	blockedChan := limiter.AttemptAccess()
	blocked := <-blockedChan

	if !blocked {
		counter := limiter.UpdateCounter(incrementReq.Inc)
		c.JSON(http.StatusOK, gin.H{"count": counter})
	} else {
		cache := limiter.UpdateCache(incrementReq.Inc)
		c.JSON(http.StatusOK, gin.H{"delayed": cache})
	}
}

type IncrementRequest struct {
	Inc int32 `json:"inc" binding:"-"`
}

// Example curl POST request:
// curl -d '{ "inc":1}' -H "Content-Type: application/json" -X POST http://localhost:8080/counter
