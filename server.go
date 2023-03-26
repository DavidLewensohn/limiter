package main

import (
	"context"
	"flag"
	"limiter/service"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// Parse command line flags
	threshold := flag.Uint("threshold", 5, "Max requests in a time period")
	ttl := flag.Uint("ttl", 10000, "Time to live in milliseconds")
	updateTime := flag.Uint("update", 10000, "Time with no post request in milliseconds to update the DB")
	flag.Parse()

	// Get a new Limiter instance with the specified threshold, ttl, and update time
	limiter := service.GetLimiter(*threshold, *ttl, *updateTime)

	// Set up the Gin router and start the server
	router := setupRouter(limiter)
	startServer(router)

	// Serve static files from the "public" directory under the "/counter" endpoint
	router.Static("/counter", "./public")
}

func setupRouter(limiter *service.Limiter) *gin.Engine {
	// Create a new Gin router with the default middleware and recovery
	router := gin.Default()

	// Add a middleware function that sets the "Access-Control-Allow-Headers" header to "Content-Type"
	router.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	})

	// Define the endpoints
	router.GET("/counter", func(c *gin.Context) {
		c.JSON(http.StatusOK, limiter.GetCounter())
	})
	router.POST("/counter", func(c *gin.Context) {
		handleCounterIncrement(c, limiter)
	})
	router.OPTIONS("/counter", func(c *gin.Context) {
		// Return a 204 No Content status code for OPTIONS requests
		c.AbortWithStatus(http.StatusNoContent)
	})

	// Return the configured router
	return router
}

func startServer(handler http.Handler) {
	// Get the port from the PORT environment variable, or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
		log.Printf("Defaulting to port %s", port)
	}

	// Create a new HTTP server with the given handler and port
	srv := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	log.Println("Starting server on port " + port)

	// Start the server in a goroutine so that it doesn't block the graceful shutdown handling below
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			// If an error occurs while serving requests, log the error and exit
			log.Fatalf("listen: %s\n", err)
		}
	}()

	// Wait for an interrupt signal to gracefully shutdown the server with a timeout of 5 seconds
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// Allow the server up to 5 seconds to shutdown gracefully
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
		return
	}

	log.Println("Server exiting")
}

// handleCounterIncrement handles the POST request to increment the counter.
// It first checks whether the client is blocked due to exceeding the limit.
func handleCounterIncrement(c *gin.Context, limiter *service.Limiter) {
	// Bind the JSON request body to the IncrementRequest struct
	var incrementReq IncrementRequest
	if err := c.ShouldBindJSON(&incrementReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Attempt to access the limiter, which returns a channel indicating whether the client is blocked or not
	blockedChan := limiter.AttemptAccess()
	blocked := <-blockedChan

	// Prepare the response struct
	resp := IncrementResponse{}

	// If not blocked, update the counter and send the new count as a response
	if !blocked {
		counter := limiter.UpdateCounter(incrementReq.Inc)
		resp.Count = counter
		log.Println("Counter:", counter)
	} else {
		// If blocked, update the cache and send the cache value as a response
		cache := limiter.UpdateCache(incrementReq.Inc)
		resp.Count = cache
	}

	// Set the IsBlocked field of the response struct based on whether the client is blocked or not
	resp.IsBlocked = blocked

	// Send the response as JSON
	c.JSON(http.StatusOK, resp)
}

// IncrementRequest represents the request body for the POST request to increment the counter.
type IncrementRequest struct {
	Inc int32 `json:"inc" binding:"required"`
}

// IncrementResponse represents the response body for the POST request to increment the counter.
type IncrementResponse struct {
	Count     int32 `json:"count"`
	IsBlocked bool  `json:"isBlocked,omitempty"`
}

// Example curl POST request:
// curl -d '{ "inc":1}' -H "Content-Type: application/json" -X POST http://localhost:8080/counter
