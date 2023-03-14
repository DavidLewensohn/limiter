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
	ttl := flag.Uint("ttl", 10000, "Time to live in miliseconds")
	updateTime := flag.Uint("update", 50000, "Time with no post request in miliseconds to update the DB")
	flag.Parse()

	limiter := service.GetLimiter(*threshold, *ttl, *updateTime)
	server := createServer(limiter)
	server.Static("/counter", "./public")
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	server.Run(":" + port)
}

func createServer(limiter service.Limiter) *gin.Engine {
	server := gin.Default()

	server.GET("/counter", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, limiter.GetCounter())
	})
	server.POST("/counter", func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		var increment service.Increment
		if err := c.ShouldBindJSON(&increment); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		blockedChan := limiter.AttemptAcces()
		blocked := <-blockedChan
		if !blocked {
			counter := limiter.UpdateCounter(increment.Inc)

			c.JSON(http.StatusOK, gin.H{"count": counter})
		} else {
			cache := limiter.UpdateCache(increment.Inc)
			c.JSON(http.StatusOK, gin.H{"delayed": cache})
		}
	})
	server.OPTIONS("/counter", func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "content-type")
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.AbortWithStatus(http.StatusNoContent)
	})
	return server
}
