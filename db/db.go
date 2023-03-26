package db

import (
	"sync/atomic"
)

// DB is an interface for the database
type DB interface {
	WriteToDb(i int32) int32
	ReadFromDb() int32
}

// db is a struct that implements the DB interface
type db struct {
	counter int32
}

// WriteToDb writes to the database
func (d *db) WriteToDb(i int32) int32 {
	atomic.AddInt32(&d.counter, i)
	return d.counter
}

// ReadFromDb reads from the database
func (d *db) ReadFromDb() int32 {
	return d.counter
}

// GetDb returns a new instance of the database
func GetDb() DB {
	db := &db{
		counter: 0,
	}
	return db
}
