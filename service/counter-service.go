package service

import (
	"fmt"
	"limiter/db"
	"time"
)

type Counter struct {
	Count int32 `json:"count"`
}
type Increment struct {
	Inc int32 `json:"inc"`
}

type Cache struct {
	cache       int32
	updateTimer time.Duration
	cacheChan   chan int32
	cacheReset  chan bool
}
type postReqEvnts struct {
	reqChan chan bool
	evntCnt uint
	timer   *time.Timer
	blocked chan bool
}

type Limiter struct {
	threshold    uint
	ttl          time.Duration
	postReqEvnts postReqEvnts
	currDb       db.DB
	cache        Cache
}

func (limiter *Limiter) GetCounter() Counter {
	var counter Counter
	counter.Count = limiter.currDb.ReadFromDb()
	return counter
}

func (limiter *Limiter) AttemptAcces() chan bool {
	limiter.postReqEvnts.reqChan <- true
	return limiter.postReqEvnts.blocked

}
func (limiter *Limiter) UpdateCounter(increment int32) int32 {
	return limiter.currDb.WriteToDb(increment)
}
func (limiter *Limiter) UpdateCache(increment int32) int32 {
	limiter.cache.cacheChan <- increment
	go func() {
		cacheReset := <-limiter.cache.cacheReset
		if cacheReset {
			limiter.cache.cache = 0
		}
	}()

	limiter.cache.cache += increment
	return limiter.cache.cache
}

func GetLimiter(threshold uint, ttl uint, updateTime uint) Limiter {
	limiter := &Limiter{
		threshold: threshold,
		ttl:       time.Duration(ttl * 1000000),
		postReqEvnts: postReqEvnts{
			timer: &time.Timer{},
		},
		currDb: db.GetDb(),
		cache: Cache{
			cache:       0,
			updateTimer: time.Duration(updateTime * 1000000),
		},
	}
	limiter.start()
	return *limiter
}
func (limiter *Limiter) start() {
	limiter.postReqEvnts.reqChan = make(chan bool, 100)
	limiter.postReqEvnts.blocked = make(chan bool)
	limiter.cache.cacheChan = make(chan int32)
	limiter.cache.cacheReset = make(chan bool)

	updateTimer := time.NewTimer(limiter.cache.updateTimer)

	go func(limiter *Limiter) {
		for {
			select {
			// The ttl-timer has finished:
			case <-limiter.postReqEvnts.timer.C:
				limiter.postReqEvnts.evntCnt = 0
				limiter.saveCache()
				limiter.cache.cache = 0
				fmt.Println("Timer expired")

			// There is POST request:
			case <-limiter.postReqEvnts.reqChan:
				// Reset the update Timer
				updateTimer = time.NewTimer(limiter.cache.updateTimer)
				// Start the ttl timer in the first request
				if limiter.postReqEvnts.evntCnt == 0 {
					limiter.postReqEvnts.timer = time.NewTimer(limiter.ttl)
					fmt.Println("timer started")
				}
				limiter.postReqEvnts.evntCnt++
				// blocked the request if there is threshold requests
				if limiter.postReqEvnts.evntCnt > limiter.threshold {
					limiter.postReqEvnts.blocked <- true
					fmt.Println("Too many requests. Blocking requests until the timer expires.")
				} else {
					limiter.postReqEvnts.blocked <- false
				}
			// Update cache if there is a limit:
			case increment := <-limiter.cache.cacheChan:
				limiter.cache.cache += increment

			// Save to cache if update-timer is done:
			case <-updateTimer.C:
				limiter.saveCache()
				limiter.cache.cache = 0
			}
		}
	}(limiter)
}

func (limiter *Limiter) saveCache() {
	limiter.UpdateCounter(limiter.cache.cache)
	limiter.cache.cacheReset <- true
}
