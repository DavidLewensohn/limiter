package service

import (
	"fmt"
	"limiter/db"
	"sync"
	"time"
)

// Limiter limits the number of requests that can be made within a specified time
type Limiter struct {
	Threshold     uint          // The maximum number of requests allowed in TTL periods
	TTL           time.Duration // The time period within which requests are allowed
	PostReqEvents PostReqEvents // A struct to hold information about post request events
	Database      db.DB         // The database used to store the counter
	Cache         Cache         // The cache used to temporarily store
	mux           sync.Mutex    // A mutex to ensure thread-safe access to the cache and database
}

// PostReqEvents holds information about the POST request events
type PostReqEvents struct {
	RequestChannel chan bool   // A channel to receive requests
	EventCnt       uint        // The number of requests made within the TTL period
	TTLTimer       *time.Timer // A timer to track the TTL period
	BlockedChan    chan bool   // A channel to block requests if the maximum number of requests is exceeded
}

// Cache represents the temporary count when access is blocked
type Cache struct {
	Value       int32
	UpdateTimer time.Duration
}

type CounterResponse struct {
	Count int32 `json:"count"`
}

// GetCounter retrieves the current counter value from the database
func (limiter *Limiter) GetCounter() CounterResponse {
	// Acquire a mutex to ensure thread-safe access to the cache and database
	limiter.mux.Lock()
	defer limiter.mux.Unlock()

	// Retrieve the current counter value from the database and assign it to the CounterResponse struct
	var counter CounterResponse
	counter.Count = limiter.Database.ReadFromDb()

	return counter
}

// AttemptAccess attempts to access the resource and blocks the request if needed.
func (limiter *Limiter) AttemptAccess() chan bool {
	// Send a request to the RequestChannel to notify the Limiter of an incoming request
	limiter.PostReqEvents.RequestChannel <- true

	// Return the BlockedChan, which will block the request if the maximum number of requests is exceeded
	return limiter.PostReqEvents.BlockedChan
}

// UpdateCounter increment the counter in the database and returns the new counter
func (limiter *Limiter) UpdateCounter(increment int32) int32 {
	limiter.mux.Lock()
	defer limiter.mux.Unlock()

	newCounter := limiter.Database.WriteToDb(increment)
	return newCounter
}

// UpdateCache updates the temporary cache
func (limiter *Limiter) UpdateCache(increment int32) int32 {
	limiter.mux.Lock()
	defer limiter.mux.Unlock()

	limiter.Cache.Value += increment // add the increment to the current cache value

	return limiter.Cache.Value // return the updated cache value
}

// GetLimiter returns a new Limiter with the specified threshold, TTL period, and update time
func GetLimiter(threshold uint, ttl uint, updateTime uint) *Limiter {
	limiter := &Limiter{
		Threshold: threshold,
		TTL:       time.Duration(ttl * 1000000),
		PostReqEvents: PostReqEvents{
			TTLTimer: &time.Timer{},
		},
		Database: db.GetDb(),
		Cache: Cache{
			Value:       0,
			UpdateTimer: time.Duration(updateTime * 1000000),
		},
	}
	limiter.start()
	return limiter
}

// Initialize the Limiter and start a Goroutine to handle limits
func (limiter *Limiter) start() {
	// create a channel to receive POST requests
	limiter.PostReqEvents.RequestChannel = make(chan bool)
	// create a channel to block requests if the maximum number of requests is exceeded
	limiter.PostReqEvents.BlockedChan = make(chan bool)

	// create a timer for updating the cache
	updateTimer := time.NewTimer(limiter.Cache.UpdateTimer)

	go func(limiter *Limiter) {
		for {
			select {
			// If the TTL timer has expired:
			case <-limiter.PostReqEvents.TTLTimer.C:
				limiter.PostReqEvents.EventCnt = 0 // reset the request count
				limiter.saveCache()                // save the current cache value to the database
				fmt.Println("TTL timer expired")

			// If there is a POST request:
			case <-limiter.PostReqEvents.RequestChannel:
				// Reset the update Timer
				updateTimer = time.NewTimer(limiter.Cache.UpdateTimer)
				fmt.Println("UPDATE timer started")

				// Start the TTL timer on the first request
				if limiter.PostReqEvents.EventCnt == 0 {
					limiter.PostReqEvents.TTLTimer = time.NewTimer(limiter.TTL)
					fmt.Println("TTL timer started")
				}
				limiter.PostReqEvents.EventCnt++ // increment the request count

				// Block the request if the threshold has been exceeded
				if limiter.PostReqEvents.EventCnt > limiter.Threshold {
					limiter.PostReqEvents.BlockedChan <- true
					fmt.Println("Too many requests. Blocking requests until the TTL timer expires.")
				} else {
					limiter.PostReqEvents.BlockedChan <- false
				}

			// Save the cache to the database if the update timer has expired
			case <-updateTimer.C:
				fmt.Println("UPDATE Timer expired")
				limiter.saveCache()
			}
		}
	}(limiter)
}

// Save the temporary cache to the database and reset the cache value
func (limiter *Limiter) saveCache() {
	limiter.mux.Lock()
	defer limiter.mux.Unlock()

	// Write the current cache value to the database
	limiter.Database.WriteToDb(limiter.Cache.Value)

	// Reset the cache value to 0
	limiter.Cache.Value = 0
}
