package main

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"net"
	"runtime/debug"

	"github.com/dgraph-io/ristretto"

	"github.com/coocood/freecache"
	"github.com/recoilme/mcproto"
	"github.com/recoilme/sniper"
)

type b52 struct {
	ssd       *sniper.Store
	lru       *ristretto.Cache
	ttl       *freecache.Cache
	udpSrv    *net.UDPConn
	udpClnAdr string
}

func udpCln(udpClnAdr string, cmd byte, k, v []byte) error {
	if udpClnAdr == "" {
		return nil
	}
	//slave
	udpAddr, err := net.ResolveUDPAddr("udp", udpClnAdr)
	if err != nil {
		return err
	}

	// Now dial to udp  at selected addr
	udpCln, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return err
	}
	defer udpCln.Close()

	// write head
	sizeb, size := sniper.NextPowerOf2(uint32(len(v) + len(k) + 8))
	b := make([]byte, size)
	b[0] = sizeb
	b[1] = cmd
	//len key in bigendian format
	lenk := uint16(len(k))
	b[2] = byte(lenk >> 8)
	b[3] = byte(lenk)
	//len val in bigendian format
	lenv := uint32(len(v))
	b[4] = byte(lenv >> 24)
	b[5] = byte(lenv >> 16)
	b[6] = byte(lenv >> 8)
	b[7] = byte(lenv)
	// write body: val and key
	copy(b[8:], v)
	copy(b[8+lenv:], k)

	_, err = udpCln.Write(b)
	return err
}

func newUDPSrv(address string, ssd *sniper.Store, lru *ristretto.Cache) (udpConn *net.UDPConn, err error) {
	// Lets prepare a udp server
	udpAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		return
	}

	// Now listen udp  at selected port
	udpConn, err = net.ListenUDP("udp", udpAddr)
	if err != nil {
		return
	}
	fmt.Printf("\nServer is listening on %s %s \n", "udp", address)

	handleConn := func(conn *net.UDPConn) {
		for {
			sizeb := make([]byte, 1)
			n, err := udpConn.Read(sizeb) //.ReadFromUDP(b)
			fmt.Println("Received ", string(sizeb[:]))
			if err != nil || n != 1 {
				fmt.Println("Error: ", err)
				conn.Close()
			}
			shift := (1 << byte(sizeb[0])) - 1
			b := make([]byte, shift)
			n, err = udpConn.Read(b)
			fmt.Println("Received ", string(b[:]))
			if err != nil || n != shift {
				fmt.Println("Error: ", err)
				conn.Close()
			}
			cmdb := b[0]
			lenk := binary.BigEndian.Uint16(b[1:3])
			lenv := binary.BigEndian.Uint32(b[3:7])
			k := b[7+lenv : 7+lenv+uint32(lenk)]
			v := b[7 : 7+lenv]
			if cmdb == byte(42) {
				//delete
				ssd.Delete(k)
				lru.Del(k)
			}
			if cmdb == byte(0) {
				//set
				ssd.Set(k, v)
				// update on lru if any
				if _, ok := lru.Get(k); ok {
					// if in LRU cache - update
					lru.Set(k, v, 0)
				}
			}
		}
	}
	go handleConn(udpConn)
	return
}

// newb52 - init filter
func newb52(params string, udpSrvAdr, udpClnAdr string) (mcproto.McEngine, error) {
	db := &b52{}
	ssd, err := sniper.Open("1")
	if err != nil {
		return nil, err
	}
	db.ssd = ssd

	x := 1024 * 1024 * 100 //100Mb
	//capacity := uint64(x)
	lru, err := ristretto.NewCache(&ristretto.Config{
		MaxCost:     int64(x),
		NumCounters: int64(x) * 10,
		BufferItems: 64,
	})
	if err != nil {
		return nil, err
	}
	db.lru = lru

	cacheSize := 100 * 1024 * 1024
	db.ttl = freecache.NewCache(cacheSize)
	debug.SetGCPercent(20)

	udpSrv, err := newUDPSrv(udpSrvAdr, ssd, lru)
	if err != nil {
		return nil, err
	}
	db.udpSrv = udpSrv

	db.udpClnAdr = udpClnAdr
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

	// update on lru if any
	if _, ok := db.lru.Get(key); ok {
		// if in LRU cache - update
		db.lru.Set(key, value, 0)
	}

	go udpCln(db.udpClnAdr, byte(0), key, value)

	return
}

func (db *b52) Incr(key []byte, value uint64, rw *bufio.ReadWriter) (result uint64, isFound bool, noreply bool, err error) {
	return
}
func (db *b52) Decr(key []byte, value uint64, rw *bufio.ReadWriter) (result uint64, isFound bool, noreply bool, err error) {
	return
}
func (db *b52) Delete(key []byte, rw *bufio.ReadWriter) (isFound bool, noreply bool, err error) {
	go udpCln(db.udpClnAdr, byte(42), key, nil)
	return
}
func (db *b52) Close() (err error) {
	err = db.ssd.Close()
	if err == nil {
		err = db.udpSrv.Close()
	}
	return
}
