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
	cold *sniper.Store
	warm *ristretto.Cache
	hot  *freecache.Cache
}

// newb52 - init filter
func newb52() (mcproto.McEngine, error) {
	db := &b52{}
	s, err := sniper.Open("1")
	if err != nil {
		return nil, err
	}
	db.cold = s

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
	db.warm = cache

	cacheSize := 100 * 1024 * 1024
	hot := freecache.NewCache(cacheSize)
	db.hot = hot
	debug.SetGCPercent(20)
	return db, nil
}

func (db *b52) Get(key []byte, rw *bufio.ReadWriter) (value []byte, noreply bool, err error) {
	if val, ok := db.warm.Get(key); ok {
		return val.([]byte), false, nil
	}
	if val, err := db.hot.Get(key); err == nil {
		return val, false, nil
	}
	value, err = db.cold.Get(key)
	return
}

func (db *b52) Gets(keys [][]byte, rw *bufio.ReadWriter) (err error) {
	return
}

func (db *b52) Set(key, value []byte, flags uint32, exp int32, size int, noreply bool, rw *bufio.ReadWriter) (noreplyresp bool, err error) {
	if exp == 0 {
		// if key pesistent (no TTL)
		err = db.cold.Set(key, value)
		if _, ok := db.warm.Get(key); ok {
			// if in LRU cache - update
			db.warm.Set(key, value, 0)
		}
	} else {
		// set in TTL cache
		err = db.hot.Set(key, value, int(exp))
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
