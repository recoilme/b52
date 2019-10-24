package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"net/url"
	"runtime"
	"runtime/debug"
	"strconv"
	"sync/atomic"

	"github.com/coocood/freecache"
	"github.com/golang/snappy"
	"github.com/recoilme/sniper"
)

type b52 struct {
	ssd    *sniper.Store
	lru    *freecache.Cache
	ttl    *freecache.Cache
	slave  net.Conn
	cmdGet uint64 // Cumulative number of retrieval reqs
	cmdSet uint64 // Cumulative number of storage reqs
	/*
	   | get_hits              | 64u     | Number of keys that have been requested   |
	   |                       |         | and found present                         |
	   | get_misses            | 64u     | Number of items that have been requested  |
	   |                       |         | and not found
	   | get_expired           | 64u     | Number of items that have been requested  |
	   |                       |         | but had already expired.                  |
	*/
}

//McEngine  - any db implemented memcache proto
type McEngine interface {
	Get(key []byte) (value []byte, err error)
	Gets(keys [][]byte) (response []byte, err error)
	Set(key, value []byte, flags uint32, exp int32, size int, noreply bool) (err error)
	Incr(key []byte, value uint64) (result uint64, err error)
	Decr(key []byte, value uint64) (result uint64, err error)
	Delete(key []byte) (isFound bool, err error)
	Close() error
	Count() uint64
	Stats() (response []byte, err error)
}

// Newb52 - init database with params
func Newb52(params, slaveadr string) (McEngine, error) {
	p, err := url.ParseQuery(params)
	if err != nil {
		log.Fatal(err)
	}
	//params
	sizelru := "100"
	if len(p["sizelru"]) > 0 {
		sizelru = p["sizelru"][0]
	}
	lrusize, err := strconv.Atoi(sizelru)
	if err != nil {
		println("sizelru parse error, fallback to default, 100Mb", err.Error())
	} else {
		println("sizelru:", lrusize, "Mb")
	}
	lrusize = lrusize * 1024 * 1024 //Mb

	sizettl := "100"
	if len(p["sizettl"]) > 0 {
		sizettl = p["sizettl"][0]
	}
	ttlsize, err := strconv.Atoi(sizettl)
	if err != nil {
		fmt.Println("sizettl parse error, fallback to default, 100Mb", err.Error())
	} else {
		println("sizettl:", ttlsize, "Mb")
	}
	ttlsize = ttlsize * 1024 * 1024

	dbdir := "db"
	if len(p["dbdir"]) > 0 {
		dbdir = p["dbdir"][0]
	}
	println("dbdir:", dbdir)

	slave := ""
	if len(slaveadr) > 0 {
		slave = slaveadr
	}

	db := &b52{}
	if dbdir == "" {
		db.ssd = nil
	} else {
		ssd, err := sniper.Open(dbdir)
		if err != nil {
			return nil, err
		}
		db.ssd = ssd
	}

	db.lru = freecache.NewCache(lrusize)

	db.ttl = freecache.NewCache(ttlsize)
	debug.SetGCPercent(20)

	if slave != "" {

		c, err := net.Dial("tcp", slave)
		if err != nil {
			panic(err)
		}
		db.slave = c
	}

	atomic.StoreUint64(&db.cmdGet, 0)
	atomic.StoreUint64(&db.cmdSet, 0)

	return db, nil
}

// Get return value from lru or ttl cache or from disk storage
func (db *b52) Get(key []byte) (value []byte, err error) {
	atomic.AddUint64(&db.cmdGet, 1)
	if val, err := db.lru.Get(key); err == nil {
		return snappy.Decode(nil, val)
	}
	if value, err := db.ttl.Get(key); err == nil {
		return snappy.Decode(nil, value)
	}
	err = nil // clear key not found err
	if db.ssd != nil {
		value, err = db.ssd.Get(key)
		if err == nil {
			value, err = snappy.Decode(nil, value)
		}
	}
	return
}

func (db *b52) Gets(keys [][]byte) (resp []byte, err error) {
	//mutex?
	buf := bytes.NewBuffer([]byte{})
	w := bufio.NewWriter(buf)
	for _, key := range keys {
		atomic.AddUint64(&db.cmdGet, 1)
		if val, err := db.lru.Get(key); err == nil {
			if val, errsn := snappy.Decode(nil, val); errsn == nil {
				fmt.Fprintf(w, "VALUE %s 0 %d\r\n%s\r\n", key, len(val), val)
			}
			continue
		}
		if value, errttl := db.ttl.Get(key); errttl == nil {
			if value, errsn := snappy.Decode(nil, value); errsn == nil {
				fmt.Fprintf(w, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
			}
			continue
		}
		if db.ssd != nil {
			if value, errssd := db.ssd.Get(key); errssd == nil {
				if value, errsn := snappy.Decode(nil, value); errsn == nil {
					fmt.Fprintf(w, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
				}
			}
		}
	}
	w.Write([]byte("END\r\n"))
	w.Flush()

	return buf.Bytes(), nil
}

// Set store k/v with expire time in memory cache
// Persistent k/v - stored on disk
func (db *b52) Set(key, value []byte, flags uint32, exp int32, size int, noreply bool) (err error) {
	//println("set", string(key), string(value))
	atomic.AddUint64(&db.cmdSet, 1)
	value = snappy.Encode(nil, value)
	if exp > 0 {
		err = db.ttl.Set(key, value, int(exp))
		return
	}
	// if key pesistent (no TTL)
	if db.ssd != nil {
		err = db.ssd.Set(key, value) // store on disk
		// update on lru if any
		if err != nil {
			return
		}
		db.lru.Set(key, value, 10)

		if db.slave != nil && flags != 42 && err == nil {
			//looks stupid
			buf := bytes.NewBuffer([]byte{})
			//w := bufio.NewWriter(buf)
			fmt.Fprintf(buf, "set %s 42 0 %d\r\n%s\r\n", key, len(value), value)
			//w.Flush()
			n, e := db.slave.Write(buf.Bytes())
			//e := db.slave.Set(&memcache.Item{Key: string(key), Value: value, Flags: 42, Expiration: exp})
			if e != nil {
				fmt.Println("slave err", e.Error(), n)
			}
		}
		return
	}
	//no disk store
	db.lru.Set(key, value, 0)
	return
}

func (db *b52) Incr(key []byte, value uint64) (result uint64, err error) {
	if db.ssd != nil {
		result, err = db.ssd.Incr(key, value)
	}
	return
}

func (db *b52) Decr(key []byte, value uint64) (result uint64, err error) {
	if db.ssd != nil {
		result, err = db.ssd.Decr(key, value)
	}
	return
}

func (db *b52) Delete(key []byte) (isFound bool, err error) {
	isFound = db.ttl.Del(key)
	db.lru.Del(key)
	if db.ssd != nil {
		isFound, err = db.ssd.Delete(key)

		if db.slave != nil && isFound && err == nil {
			//looks stupid
			buf := bytes.NewBuffer([]byte{})
			w := bufio.NewWriter(buf)

			fmt.Fprintf(w, "delete %s\r\n", key)
			w.Flush()
			n, e := db.slave.Write(buf.Bytes())
			if e != nil {
				fmt.Println("slave err", e.Error(), n)
			}
		}
	}
	return
}

func (db *b52) Close() (err error) {
	if db.ssd != nil {
		err = db.ssd.Close()
	}

	return
}

func (db *b52) Count() uint64 {
	if db.ssd != nil {
		return uint64(db.ssd.Count())
	}
	return 0
}

func (db *b52) Stats() (resp []byte, err error) {
	ver := "STAT version " + version + "\r\n"
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	bytes := fmt.Sprintf("bytes %d\r\n", ms.Sys)
	total := fmt.Sprintf("heap_sys_mb %d\r\n", ms.HeapSys/1024/1024)
	currItems := fmt.Sprintf("curr_items %d\r\n", db.Count())
	cmdGet := fmt.Sprintf("cmd_get %d\r\n", atomic.LoadUint64(&db.cmdGet))
	cmdSet := fmt.Sprintf("cmd_set %d\r\n", atomic.LoadUint64(&db.cmdSet))
	return []byte(ver + bytes + total + currItems + cmdGet + cmdSet + "END\r\n"), nil
}
