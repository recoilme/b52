package main

import (
	"bufio"
	"fmt"
	"log"
	"net/url"
	"runtime/debug"
	"strconv"
	"sync"

	"github.com/dgraph-io/ristretto"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/coocood/freecache"
	"github.com/recoilme/mcproto"
	"github.com/recoilme/sniper"
)

type b52 struct {
	ssd   *sniper.Store
	lru   *ristretto.Cache
	ttl   *freecache.Cache
	slave string
}

// newb52 - init filter
func newb52(params, slaveadr string) (mcproto.McEngine, error) {
	p, err := url.ParseQuery(params)
	if err != nil {
		log.Fatal(err)
	}
	//params
	sizelru := "100"
	if len(p["sizelru"]) > 0 {
		sizelru = p["sizelru"][0]
	} else {
		fmt.Println("sizelru not set, fallback to default")
	}
	lrusize, err := strconv.Atoi(sizelru)
	if err != nil {
		fmt.Println("sizelru parse error, fallback to default", err.Error())
		log.Fatal(err)
	}
	lrusize = lrusize * 1024 * 1024 //Mb

	sizettl := "100"
	if len(p["sizettl"]) > 0 {
		sizettl = p["sizettl"][0]
	} else {
		fmt.Println("sizettl not set, fallback to default")
	}
	ttlsize, err := strconv.Atoi(sizettl)
	if err != nil {
		fmt.Println("sizettl parse error, fallback to default", err.Error())
		log.Fatal(err)
	}
	ttlsize = ttlsize * 1024 * 1024

	dbdir := "db"
	if len(p["dbdir"]) > 0 {
		dbdir = p["dbdir"][0]
	}

	slave := ""
	if len(slaveadr) > 0 {
		slave = slaveadr
	}

	db := &b52{}
	ssd, err := sniper.Open(dbdir)
	if err != nil {
		return nil, err
	}
	db.ssd = ssd

	lru, err := ristretto.NewCache(&ristretto.Config{
		MaxCost:     int64(lrusize),
		NumCounters: int64(lrusize) * 10,
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}
	db.lru = lru

	db.ttl = freecache.NewCache(ttlsize)
	debug.SetGCPercent(20)

	db.slave = slave
	println("set slave at:", db.slave)

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
	var wg sync.WaitGroup
	read := func(key []byte) {
		defer wg.Done()
		//var value []byte
		if val, ok := db.lru.Get(key); ok {
			fmt.Fprintf(rw, "VALUE %s 0 %d\r\n%s\r\n", key, len(val.([]byte)), val.([]byte))
			return //val.([]byte), false, nil
		}
		if value, err := db.ttl.Get(key); err == nil {
			fmt.Fprintf(rw, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
			return
		}
		value, err := db.ssd.Get(key)
		if err == nil {
			fmt.Fprintf(rw, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
		}
	}
	wg.Add(len(keys))
	for _, key := range keys {
		go read(key)
	}
	wg.Wait()
	_, err = rw.Write([]byte("END\r\n"))
	if err == nil {
		err = rw.Flush()
	}
	return err

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

	// update on lru if any
	if _, ok := db.lru.Get(key); ok {
		// if in LRU cache - update
		db.lru.Set(key, value, 0)
	}

	if db.slave != "" && flags != 1 {
		mc := memcache.New(db.slave)
		mc.MaxIdleConns = 1
		errSlave := mc.Set(&memcache.Item{Key: string(key), Value: value, Flags: 1, Expiration: exp})
		if errSlave != nil {
			println(err.Error())
		}
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
	err = db.ssd.Close()
	if db.slave != "" {
		println("Close")
	}

	return
}
