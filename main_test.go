package main

import (
	"fmt"
	"log"
	"net"
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/recoilme/mcproto"
)

//go test -race -run=Base
func Test_Base(t *testing.T) {
	addr := ":11211"
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("failed to serve: %s", err.Error())
		return
	}
	var b52 mcproto.McEngine
	b52, err = Newb52("sizelru=10&sizettl=90&dbdir=tst", "")
	if err != nil {
		log.Fatalf("failed to create database: %s", err.Error())
	}
	defer listener.Close()
	serve(listener, b52, "buf=4096&deadline=60001")

	// 2 clients
	mc := memcache.New(addr)
	mc.Set(&memcache.Item{Key: string("key"), Value: []byte("value"), Flags: 1, Expiration: 0})
	v, err := mc.Get("key")
	if err != nil {
		t.Error(err)
	}
	if string(v.Value) != "value" {
		t.Error("not value")
	}

	mc2 := memcache.New(addr)
	v2, err := mc2.Get("key")
	if err != nil {
		t.Error(err)
	}
	if string(v2.Value) != "value" {
		t.Error("not value")
	}

	//multi set from clients
	println("multi set from clients")
	multisize := 500
	for i := 0; i <= multisize; i++ {
		s := fmt.Sprintf("%d", i)
		item := &memcache.Item{Key: s, Value: []byte(s), Flags: 0, Expiration: 0}
		if i%2 == 0 {
			err = mc.Set(item)
		} else {
			err = mc2.Set(item)
		}
		if err != nil {
			t.Error("multiset", err)
		}
	}
	//Get
	for i := 0; i <= multisize; i++ {
		s := fmt.Sprintf("%d", i)
		var val *memcache.Item
		var geterr error
		if i%2 == 0 {
			val, geterr = mc.Get(s)
		} else {
			val, geterr = mc2.Get(s)
		}
		if geterr != nil {
			t.Error(err)
		}
		if string(val.Value) != s {
			t.Error("bad news")
		} else {
			//println(string(val.Value))
		}
	}

	// Multiget

	multi := make([]string, multisize+1)
	for i := 0; i <= multisize; i++ {
		multi[i] = fmt.Sprintf("%d", i)
	}

	t2 := time.Now()
	items, err := mc.GetMulti(multi)
	t3 := time.Now()
	if t3.Sub(t2) > (1 * time.Millisecond) {
		println("multiget is slow:", t3.Sub(t2).Milliseconds())
	}
	if err != nil {
		t.Error("multiget", err)
	}
	if len(items) != multisize+1 {
		t.Error("bad news", len(items))
	}

	for key, item := range items {
		if key != string(item.Value) {
			t.Error("bad news", key, string(item.Value))
		}
	}

	//close
	err = b52.Close()
	if err != nil {
		t.Error(err)
	}

}
