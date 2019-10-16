package main

import (
	"bufio"
	"bytes"
	"fmt"
	"log"
	"net"
	"net/url"
	"runtime/debug"
	"strconv"

	"github.com/dgraph-io/ristretto"

	"github.com/coocood/freecache"
	"github.com/recoilme/sniper"
)

type b52 struct {
	ssd   *sniper.Store
	lru   *ristretto.Cache
	ttl   *freecache.Cache
	slave net.Conn
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

	if slave != "" {

		c, err := net.Dial("tcp", slave)
		if err != nil {
			panic(err)
		}
		db.slave = c
	}

	return db, nil
}

// Get return value from lru or ttl cache or from disk storage
func (db *b52) Get(key []byte) (value []byte, err error) {
	if val, ok := db.lru.Get(key); ok {
		return val.([]byte), nil
	}
	if value, err := db.ttl.Get(key); err == nil {
		return value, nil
	}
	err = nil // clear key not found err
	if db.ssd != nil {
		value, err = db.ssd.Get(key)
	}
	return
}

func (db *b52) Gets(keys [][]byte) (resp []byte, err error) {
	//mutex?
	buf := bytes.NewBuffer([]byte{})
	w := bufio.NewWriter(buf)
	for _, key := range keys {
		if val, ok := db.lru.Get(key); ok {
			fmt.Fprintf(w, "VALUE %s 0 %d\r\n%s\r\n", key, len(val.([]byte)), val.([]byte))
			continue
		}
		if value, errttl := db.ttl.Get(key); errttl == nil {
			fmt.Fprintf(w, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
			continue
		}
		if db.ssd != nil {
			if value, errssd := db.ssd.Get(key); errssd == nil {
				fmt.Fprintf(w, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
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
	if exp > 0 {
		err = db.ttl.Set(key, value, int(exp))
		return
	}
	// if key pesistent (no TTL)
	if db.ssd != nil {
		err = db.ssd.Set(key, value) // store on disk
		// update on lru if any
		if _, ok := db.lru.Get(key); ok {
			// if in LRU cache - update
			db.lru.Set(key, value, 0)
		}
		if db.slave != nil && flags != 42 {
			//looks stupid
			buf := bytes.NewBuffer([]byte{})
			w := bufio.NewWriter(buf)

			fmt.Fprintf(w, "set %s 42 0 %d\r\n%s\r\n", key, len(value), value)
			w.Flush()
			n, e := db.slave.Write(buf.Bytes())
			//e := db.slave.Set(&memcache.Item{Key: string(key), Value: value, Flags: 42, Expiration: exp})
			if e != nil {
				println("slave err", e.Error(), n)
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
		if isFound {
			if db.slave != nil {
				//looks stupid
				//go db.slave.Delete(string(key))
				//errSlave := mc.Set(&memcache.Item{Key: string(key), Value: value, Flags: 1, Expiration: exp})
				//if errSlave != nil {
				//println(err.Error())
				//}
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
