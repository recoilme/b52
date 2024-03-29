package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/golang/snappy"
	"github.com/recoilme/sniper"
)

const fileMode = 0644

type accumulator struct {
	sync.RWMutex
	buf         bytes.Buffer
	accumulated int
}

type b52 struct {
	ssd *sniper.Store
	//slave     net.Conn
	slaveAddr  string
	cmdGet     uint64 // Cumulative number of retrieval reqs
	cmdSet     uint64 // Cumulative number of storage reqs
	lastUpdate int64  // last update timestamp
	accum      *accumulator
	/*
	   | get_hits              | 64u     | Number of keys that have been requested   |
	   |                       |         | and found present                         |
	   | get_misses            | 64u     | Number of items that have been requested  |
	   |                       |         | and not found
	   | get_expired           | 64u     | Number of items that have been requested  |
	   |                       |         | but had already expired.                  |
	*/
}

// McEngine  - any db implemented memcache proto
type McEngine interface {
	Get(key []byte) (value []byte, err error)
	Gets(keys [][]byte) (response []byte, err error)
	Set(key, value []byte, flags uint32, exp uint32, size int, noreply bool) (err error)
	Touch(key []byte, exp uint32) (err error)
	Incr(key []byte, value uint64) (result uint64, err error)
	Decr(key []byte, value uint64) (result uint64, err error)
	Delete(key []byte) (isFound bool, err error)
	Close() error
	Count() uint64
	Stats() (response []byte, err error)
	Backup(name string) error
	Restore(name string) error
}

// Newb52 - init database with params
func Newb52(params, slaveadr string) (McEngine, error) {
	p, err := url.ParseQuery(params)
	if err != nil {
		log.Fatal(err)
	}

	dbdir := "db"
	if len(p["dbdir"]) > 0 {
		dbdir = p["dbdir"][0]
	}
	//println("dbdir:", dbdir)

	db := &b52{}
	db.slaveAddr = slaveadr
	if dbdir == "" {
		db.ssd = nil
	} else {
		ssd, err := sniper.Open(sniper.Dir(dbdir), sniper.ExpireInterval(time.Minute))
		if err != nil {
			return nil, err
		}
		db.ssd = ssd
	}

	debug.SetGCPercent(20)

	atomic.StoreUint64(&db.cmdGet, 0)
	atomic.StoreUint64(&db.cmdSet, 0)

	db.accum = &accumulator{}

	return db, nil
}

// Get value from disk storage
func (db *b52) Get(key []byte) (value []byte, err error) {
	atomic.AddUint64(&db.cmdGet, 1)
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
func (db *b52) Set(key, value []byte, flags uint32, exp uint32, size int, noreply bool) (err error) {
	//println("set", string(key), string(value))
	atomic.AddUint64(&db.cmdSet, 1)
	if flags != 42 { //get from replication, allready encoded
		value = snappy.Encode(nil, value)
	}

	// if key pesistent (no TTL)
	if db.ssd != nil {
		atomic.StoreInt64(&db.lastUpdate, time.Now().Unix())
		expire := exp
		if exp != 0 {
			expire += uint32(time.Now().Unix())
		}
		err = db.ssd.Set(key, value, expire) // store on disk
		/*if db.slaveAddr != "" && db.slave == nil {
			//dial to slave
			c, errSlave := net.Dial("udp", db.slaveAddr)
			if errSlave != nil {
				fmt.Println(errSlave)
				return
			}
			db.slave = c
		}
		if db.slave != nil*/
		//fmt.Println(db.accum.accumulated)
		if db.slaveAddr != "" && flags != 42 && err == nil {
			db.accum.Lock()
			fmt.Fprintf(&db.accum.buf, "set %s 42 %d %d noreply\r\n%s\r\n", key, exp, len(value), value)
			db.accum.accumulated++
			if db.accum.accumulated == 200 {
				slaves := strings.Split(db.slaveAddr, ",")
				bin := db.accum.buf.Bytes()
				db.accum.buf.Reset()
				//fmt.Printf("Accum:2, buf:%s len:%d\n", string(bin), len(db.accum.buf.Bytes()))
				hostname, errHN := os.Hostname()
				if err != nil {
					fmt.Println("Error, get hostname: ", errHN)
				}
				for _, slave := range slaves {
					if strings.Contains(slave, hostname) {
						continue
					}
					c, cErr := net.Dial("tcp", slave)
					if cErr != nil {
						fmt.Println("Error, connection:" + slave + cErr.Error())
						break
					}
					writed, copyErr := io.Copy(c, bytes.NewBuffer(bin))
					if writed != int64(len(bin)) || copyErr != nil {
						fmt.Printf("Error, write 2 connection:%s writed:%d len:%d err:%v\n", slave, writed, len(bin), copyErr)
					}
					closeErr := c.Close()
					if closeErr != nil {
						fmt.Println("Error,close:" + closeErr.Error())
					}
				}
				db.accum.accumulated = 0
			}
			db.accum.Unlock()

			//go fmt.Fprintf(db.slave, "set %s 42 0 %d\r\n%s\r\n", key, len(value), value)
			//if e != nil {
			//fmt.Println("slave err", e.Error(), n)
			//db.slave = nil
			//}
		}
		return
	}
	return
}

func (db *b52) Touch(key []byte, exp uint32) (err error) {
	if db.ssd != nil {
		atomic.StoreInt64(&db.lastUpdate, time.Now().Unix())
		if exp != 0 {
			exp += uint32(time.Now().Unix())
		}
		err = db.ssd.Touch(key, exp)
	}
	return
}

func (db *b52) Incr(key []byte, value uint64) (result uint64, err error) {
	if db.ssd != nil {
		atomic.StoreInt64(&db.lastUpdate, time.Now().Unix())
		result, err = db.ssd.Incr(key, value)
	}
	return
}

func (db *b52) Decr(key []byte, value uint64) (result uint64, err error) {
	if db.ssd != nil {
		atomic.StoreInt64(&db.lastUpdate, time.Now().Unix())
		result, err = db.ssd.Decr(key, value)
	}
	return
}

func (db *b52) Delete(key []byte) (isFound bool, err error) {
	if db.ssd != nil {
		atomic.StoreInt64(&db.lastUpdate, time.Now().Unix())
		isFound, err = db.ssd.Delete(key)

		if db.slaveAddr != "" && isFound && err == nil {
			slaves := strings.Split(db.slaveAddr, ",")
			for _, slave := range slaves {
				c, cErr := net.Dial("tcp", slave)
				if cErr != nil {
					fmt.Println("Error, connection:" + slave + cErr.Error())
					break
				}
				n, e := fmt.Fprintf(c, "delete %s noreply\r\n", key)
				if e != nil {
					fmt.Println("slave err", e.Error(), n)
				}
				closeErr := c.Close()
				if closeErr != nil {
					fmt.Println("Error,close:" + closeErr.Error())
				}
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
	sniperver := "STAT sniper_version " + sniper.Version + "\r\n"
	uptime := fmt.Sprintf("STAT uptime %d\r\n", uint32(time.Now().Unix()-startTime))
	// For info on each, see: https://golang.org/pkg/runtime/#MemStats
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	sys := fmt.Sprintf("STAT bytes %d\r\n", ms.Sys)
	total := fmt.Sprintf("STAT heap_sys_mb %d\r\n", ms.HeapSys/1024/1024)
	currItems := fmt.Sprintf("STAT curr_items %d\r\n", db.Count())
	cmdGet := fmt.Sprintf("STAT cmd_get %d\r\n", atomic.LoadUint64(&db.cmdGet))
	cmdSet := fmt.Sprintf("STAT cmd_set %d\r\n", atomic.LoadUint64(&db.cmdSet))
	lastUpdate := fmt.Sprintf("STAT last_update_time %d\r\n", atomic.LoadInt64(&db.lastUpdate))
	fs := int64(0)
	if db.ssd != nil {
		fs, _ = db.ssd.FileSize()
	}

	cmdFs := fmt.Sprintf("STAT file_size %d\r\n", fs)
	/*
		buf := bytes.NewBuffer([]byte{})
		w := bufio.NewWriter(buf)
		fmt.Fprintf(w, "expvar {")
		first := true
		expvar.Do(func(kv expvar.KeyValue) {
			if !first {
				fmt.Fprintf(w, ",")
			}
			first = false
			fmt.Fprintf(w, "%q: %s", kv.Key, kv.Value)
		})
		fmt.Fprintf(w, "}\r\n")
		w.Flush()
	*/

	return []byte(ver + sniperver + uptime + sys + total + currItems + cmdGet + cmdSet + lastUpdate + cmdFs + "END\r\n"), nil
}

func (db *b52) Backup(name string) error {
	if db.ssd != nil {
		file, err := os.OpenFile(name, os.O_CREATE|os.O_WRONLY|os.O_EXCL, os.FileMode(fileMode))
		if err != nil {
			return err
		}
		defer file.Close()
		return db.ssd.Backup(file)
	}
	return nil
}

func (db *b52) Restore(name string) error {
	if db.ssd != nil {
		file, err := os.OpenFile(name, os.O_RDONLY, os.FileMode(fileMode))
		if err != nil {
			return err
		}
		defer file.Close()
		return db.ssd.Restore(file)
	}
	return nil
}
