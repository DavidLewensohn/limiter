package service

import (
	"fmt"
	"limiter/db"
	"sync"
	"time"
)

type Counter struct {
	Count int32 `json:"count"`
}

// Cache represents the temporary count when access is blocked
type Cache struct {
	Value       int32
	UpdateTimer time.Duration
}

// postReqEvents holds information about the POST request events
type PostReqEvents struct {
	RequestChannel chan bool   // A channel to receive requests
	EventCnt       uint        // The number of requests made within the TTL period
	TTLTimer       *time.Timer // A timer to track the TTL period
	BlockedChan    chan bool   // A channel to block requests if the maximum number of requests is exceeded
}

// Limiter limits the number of requests that can be made within a specified time
type Limiter struct {
	Threshold     uint          // The maximum number of requests allowed in TTL periods
	TTL           time.Duration // The time period within which requests are allowed
	PostReqEvents PostReqEvents // A struct to hold information about post request events
	Database      db.DB         // The database used to store the counter
	Cache         Cache         // The cache used to temporarily store
	mux           sync.Mutex    // A mutex to ensure thread-safe access to the cache and database
}

// GetCounter returns the current counter from database
func (limiter *Limiter) GetCounter() Counter {
	var counter Counter
	counter.Count = limiter.Database.ReadFromDb()
	return counter
}

// Attempts to access the resource, and blocks the request if needed
func (limiter *Limiter) AttemptAccess() chan bool {
	limiter.PostReqEvents.RequestChannel <- true

	return limiter.PostReqEvents.BlockedChan

}

// UpdateCounter updates the count of requests in the database
func (limiter *Limiter) UpdateCounter(increment int32) int32 {
	limiter.mux.Lock()
	defer limiter.mux.Unlock()
	return limiter.Database.WriteToDb(increment)
}

// UpdateCache updates the temporary cache
func (limiter *Limiter) UpdateCache(increment int32) int32 {
	limiter.mux.Lock()
	defer limiter.mux.Unlock()
	limiter.Cache.Value += increment
	return limiter.Cache.Value
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
func (limiter *Limiter) start() {
	limiter.PostReqEvents.RequestChannel = make(chan bool, 100)
	limiter.PostReqEvents.BlockedChan = make(chan bool)

	updateTimer := time.NewTimer(limiter.Cache.UpdateTimer)

	go func(limiter *Limiter) {
		for {
			select {
			// The TTL timer has expired:
			case <-limiter.PostReqEvents.TTLTimer.C:
				limiter.PostReqEvents.EventCnt = 0
				limiter.saveCache()
				fmt.Println("Timer expired")

				// There is POST request:
			case <-limiter.PostReqEvents.RequestChannel:
				// Reset the update Timer
				updateTimer = time.NewTimer(limiter.Cache.UpdateTimer)
				fmt.Println("UPDATE timer started")
				// Start the ttl timer in the first request
				if limiter.PostReqEvents.EventCnt == 0 {
					limiter.PostReqEvents.TTLTimer = time.NewTimer(limiter.TTL)
					fmt.Println("timer started")
				}
				limiter.PostReqEvents.EventCnt++
				// blocked the request if there is threshold requests
				if limiter.PostReqEvents.EventCnt > limiter.Threshold {
					limiter.PostReqEvents.BlockedChan <- true
					fmt.Println("Too many requests. Blocking requests until the timer expires.")
				} else {
					limiter.PostReqEvents.BlockedChan <- false
				}

			// Save to cache if update-timer is done:
			case <-updateTimer.C:
				fmt.Println("UPDATE Timer expired")
				limiter.saveCache()
			}
		}
	}(limiter)
}

func (limiter *Limiter) saveCache() {
	limiter.mux.Lock()
	defer limiter.mux.Unlock()
	limiter.Database.WriteToDb(limiter.Cache.Value)

	limiter.Cache.Value = 0
}
