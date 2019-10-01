package main

import (
	"bufio"
	"runtime/debug"

	"github.com/dgraph-io/ristretto"

	"github.com/coocood/freecache"
	"github.com/recoilme/mcproto"
	"github.com/recoilme/sniper"
)

type b52 struct {
	ssd *sniper.Store
	lru *ristretto.Cache
	ttl *freecache.Cache
}

// newb52 - init filter
func newb52() (mcproto.McEngine, error) {
	db := &b52{}
	ssd, err := sniper.Open("1")
	if err != nil {
		return nil, err
	}
	db.ssd = ssd

	x := 1024 * 1024 * 100 //100Mb
	//capacity := uint64(x)
	cache, err := ristretto.NewCache(&ristretto.Config{
		MaxCost:     int64(x),
		NumCounters: int64(x) * 10,
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}
	db.lru = cache

	cacheSize := 100 * 1024 * 1024
	db.ttl = freecache.NewCache(cacheSize)
	debug.SetGCPercent(20)
	return db, nil
}

// Get return value from lru or ttl cache or from disk storage
func (db *b52) Get(key []byte, rw *bufio.ReadWriter) (value []byte, noreply bool, err error) {
	if val, ok := db.lru.Get(key); ok {
		return val.([]byte), false, nil
	}
	if value, err := db.ttl.Get(key); err == nil {
		return value, false, nil
	}
	err = nil // clear key not found

	value, err = db.ssd.Get(key)
	return
}

func (db *b52) Gets(keys [][]byte, rw *bufio.ReadWriter) (err error) {
	return
}

// Set store k/v with expire time in memory cache
// Persistent k/v - stored on disk
func (db *b52) Set(key, value []byte, flags uint32, exp int32, size int, noreply bool, rw *bufio.ReadWriter) (noreplyresp bool, err error) {
	if exp > 0 {
		err = db.ttl.Set(key, value, int(exp))
		return
	}
	// if key pesistent (no TTL)
	err = db.ssd.Set(key, value) // store on disk
	if _, ok := db.lru.Get(key); ok {
		// if in LRU cache - update
		db.lru.Set(key, value, 0)
	}

	return
}
func (db *b52) Incr(key []byte, value uint64, rw *bufio.ReadWriter) (result uint64, isFound bool, noreply bool, err error) {
	return
}
func (db *b52) Decr(key []byte, value uint64, rw *bufio.ReadWriter) (result uint64, isFound bool, noreply bool, err error) {
	return
}
func (db *b52) Delete(key []byte, rw *bufio.ReadWriter) (isFound bool, noreply bool, err error) {
	return
}
func (db *b52) Close() (err error) {
	return
}
