package main

import (
	"bufio"
	"fmt"
	"log"
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
	slave string
}

//McEngine  - any db implemented memcache proto
type McEngine interface {
	Get(key []byte, rw *bufio.ReadWriter) (value []byte, noreply bool, err error)
	Gets(keys [][]byte, rw *bufio.Writer) (keysvals [][]byte, err error)
	Set(key, value []byte, flags uint32, exp int32, size int, noreply bool, rw *bufio.ReadWriter) (noreplyresp bool, err error)
	Incr(key []byte, value uint64, rw *bufio.ReadWriter) (result uint64, isFound bool, noreply bool, err error)
	Decr(key []byte, value uint64, rw *bufio.ReadWriter) (result uint64, isFound bool, noreply bool, err error)
	Delete(key []byte, rw *bufio.ReadWriter) (isFound bool, noreply bool, err error)
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

	db.slave = slave
	if slave != "" {
		println("set slave at:", db.slave)
	}

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
	if db.ssd != nil {
		value, err = db.ssd.Get(key)
	}
	return
}

func (db *b52) Gets(keys [][]byte, rw *bufio.Writer) (kv [][]byte, err error) {
	//t2 := time.Now()
	/*
		var wg sync.WaitGroup

		//response := make(chan [][]byte)
		read := func(key []byte) {
			defer wg.Done()

			//res := make([][]byte, 0)
			if val, ok := db.lru.Get(key); ok {
				//res = append(res, key)
				//res = append(res, val.([]byte))
				//response <- res
				//	fmt.Fprintf(b, "VALUE %s 0 %d\r\n%s\r\n", key, len(val.([]byte)), val.([]byte))
				//bufPool.Put(b)
				_ = val
				return //val.([]byte), false, nil
			}
			if value, errttl := db.ttl.Get(key); errttl == nil {
				//res = append(res, key)
				//res = append(res, value)
				//response <- res
				//fmt.Fprintf(b, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
				//bufPool.Put(b)
				_ = value
				return
			}
			value, errssd := db.ssd.Get(key)
			if errssd == nil {
				_ = value
				//res = append(res, key)
				//	res = append(res, value)
				//response <- res
				//fmt.Fprintf(b, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
				//bufPool.Put(b)
			}
		}
		wg.Add(len(keys))
	*/
	for _, key := range keys {
		if val, ok := db.lru.Get(key); ok {
			fmt.Fprintf(rw, "VALUE %s 0 %d\r\n%s\r\n", key, len(val.([]byte)), val.([]byte))
			continue
		}
		if value, errttl := db.ttl.Get(key); errttl == nil {
			fmt.Fprintf(rw, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
			continue
		}
		if db.ssd != nil {
			if value, errssd := db.ssd.Get(key); errssd == nil {
				fmt.Fprintf(rw, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
			}
		}
	}

	//wg.Wait()
	_, err = rw.Write([]byte("END\r\n"))
	if err != nil {
		fmt.Println("rw.Write", err.Error())
	}
	err = rw.Flush()
	if err != nil {
		fmt.Println("rw.Flush", err.Error())
	}
	//close(response)
	//}()
	/*done := make(chan struct{})
	go func() {
		defer close(done)
		for resp := range response {
			kv = append(kv, resp...)
		}
	}()

	<-done*/

	//t3 := time.Now()
	//if t3.Sub(t2) > (1 * time.Millisecond) {
	//	println("get is slow!:", t3.Sub(t2).Milliseconds())
	//}
	return kv, nil

}

// Set store k/v with expire time in memory cache
// Persistent k/v - stored on disk
func (db *b52) Set(key, value []byte, flags uint32, exp int32, size int, noreply bool, rw *bufio.ReadWriter) (noreplyresp bool, err error) {

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
	} else {
		//no disk store
		db.lru.Set(key, value, 0)
	}

	return
}

func (db *b52) Incr(key []byte, value uint64, rw *bufio.ReadWriter) (result uint64, isFound bool, noreply bool, err error) {
	result, err = db.ssd.Incr(key, value)
	return
}

func (db *b52) Decr(key []byte, value uint64, rw *bufio.ReadWriter) (result uint64, isFound bool, noreply bool, err error) {
	result, err = db.ssd.Decr(key, value)
	return
}

func (db *b52) Delete(key []byte, rw *bufio.ReadWriter) (isFound bool, noreply bool, err error) {
	db.ttl.Del(key)
	db.lru.Del(key)
	isFound, err = db.ssd.Delete(key)
	return
}

func (db *b52) Close() (err error) {
	if db.ssd != nil {
		err = db.ssd.Close()
	}
	if db.slave != "" {
		println("Close")
	}

	return
}
