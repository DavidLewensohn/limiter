package db

import (
	"sync/atomic"
)

type DB interface {
	WriteToDb(i int32) int32
	ReadFromDb() int32
}

type db struct {
	counter int32
}

func (d *db) WriteToDb(i int32) int32 {
	atomic.AddInt32(&d.counter, i)
	return d.counter
}

func (d *db) ReadFromDb() int32 {
	return d.counter
}

func GetDb() DB {
	db := &db{
		counter: 0,
	}
	return db
}
