package main

import (
	"bufio"
	"bytes"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/golang/snappy"
)

func Test_Snappy(t *testing.T) {
	inp := "585379056,1.00,802149793,2.00,604648284,3.00,601222508,4.00,802025401,5.00,801932737,6.00,801894681,7.00,801955055,8.00,802149859,9.00,802149817,10.00,802149797,11.00,802149796,12.00,802149790,13.00,802139540,14.00,802139492,15.00,802139481,16.00,802139482,17.00,802139476,18.00,587525696,19.00,802126615,20.00,802126591,21.00,802126586,22.00,802126589,23.00,802126588,24.00,802126582,25.00,802115299,26.00,802115271,27.00,802115239,28.00,802115230,29.00,802115226,30.00,802115227,31.00,802102314,32.00,802102316,33.00,802102307,34.00,802102264,35.00,802102268,36.00,802102257,37.00,802094700,38.00,802090092,39.00,802090081,40.00,802090063,41.00,802090001,42.00,802090006,43.00,802080997,44.00,802078308,45.00,802078230,46.00,802078203,47.00,802078200,48.00,802078196,49.00,802078198,50.00,802077276,51.00,802066218,52.00,802066194,53.00,802066185,54.00,802066173,55.00,802066174,56.00,801784301,57.00,801829311,58.00,802057093,59.00,802057089,60.00,802057077,61.00,802057067,62.00,802057068,63.00,802057071,64.00,802048517,65.00,802048516,66.00,802048512,67.00,802048509,68.00,802048510,69.00,802042054,70.00,802042022,71.00,802042021,72.00,802042020,73.00,802042017,74.00,802037155,75.00,802037147,76.00,802037146,77.00,802037138,78.00,668973475,79.00,802033598,80.00,802033595,81.00,802033594,82.00,802031429,83.00,802031428,84.00,802031427,85.00,802028446,86.00,802021865,86.10,802028444,87.00,802021863,87.15,802028442,88.00,802016927,88.20,802028441,89.00,802016923,89.25,802028438,90.00,802016916,90.30,617935134,91.00,802016908,91.35,802025428,92.00,802016905,92.40,802025404,93.00"
	got := snappy.Encode(nil, []byte(inp))
	println(len(inp), len(got))
	ret, err := snappy.Decode(nil, got)
	if err != nil {
		panic(err)
	}
	if !bytes.Equal([]byte(inp), ret) {
		panic("bytes.Equal")
	}
	s := fmt.Sprintf("%d", 10)
	gs := snappy.Encode(nil, []byte(s))
	println(len(s), len(gs))
}

func Test_Udp(t *testing.T) {
	addr := ":11211"
	//udp
	/*
		udpAddr, err := net.ResolveUDPAddr("udp", addr)
		if err != nil {
			panic(err)
		}
		//localUdp, err := net.ResolveUDPAddr("udp", ":0")
		//if err != nil {
		//panic(err)
		//}
		udpConn, err := net.DialUDP("udp", nil, udpAddr)
		if err != nil {
			panic(err)
		}
		defer udpConn.Close()
		buf := []byte("set e 0 0 1\r\n1\r\nset f 0 0 1\r\n2\r\n")
		_, err = udpConn.Write(buf)
		if err != nil {
			panic(err)
		}*/
	conn, err := net.Dial("udp", addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Call the `Write()` method of the implementor
	// of the `io.Writer` interface.
	_, err = fmt.Fprintf(conn, "set e 0 0 1\r\n1\r\nset f 0 0 1\r\n2\r\n")
	if err != nil {
		panic(err)
	}
}

//go test -race -run=Base
func Test_Base(t *testing.T) {
	go main()
	time.Sleep(10 * time.Second)
	addr := ":11211"
	//pipeline

	c, err := net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	rd := bufio.NewReader(c)
	w := bufio.NewWriter(c)

	fmt.Fprintf(w, "set c 0 0 1\r\n1\r\nset d 0 0 1\r\n2\r\n")
	err = w.Flush()
	if err != nil {
		panic(err)
	}
	p := make([]byte, 1024)
	n, err := rd.Read(p)
	if err != nil {
		panic(err)
	}

	if n != 16 {
		println(n, len(p))
		panic("err pipelining")
	}
	Test_Udp(t)
	// 2 clients
	mc := memcache.New(addr)
	multisize := 10000
	mc.Timeout = 1000 * time.Millisecond
	mc.Set(&memcache.Item{Key: string("key"), Value: []byte("value"), Flags: 1, Expiration: 0})
	v, err := mc.Get("key")
	if err != nil {

		panic(err)
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

	items, err := mc.GetMulti([]string{"c", "d", "e", "f"})
	if err != nil {
		panic(err)
	}
	for _, it := range items {
		_ = it
		//println(it.Key, string(it.Value))
	}

	//multi set from clients
	//println("multi set from clients")

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
	items, err = mc.GetMulti(multi)
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
			panic("'" + string(item.Value) + "'")
		}
	}

	v, err = mc.Get("not exists")
	if err == nil {
		t.Error("must be not exist")
	}

	err = mc.Set(&memcache.Item{Key: string("exists"), Value: []byte("exists"), Flags: 0, Expiration: 0})
	if err != nil {
		t.Error("must be no err")
	}
	keys := make([]string, 0)
	keys = append(keys, "some1")
	keys = append(keys, "some2")
	items, err = mc.GetMulti(keys)
	if err != nil {
		t.Error("must be no err")
	}
	err = mc.Set(&memcache.Item{Key: string("exists"), Value: []byte("exists2"), Flags: 0, Expiration: 0})
	if err != nil {
		t.Error("must be no err")
	}
	keys = append(keys, "exists")
	items, err = mc.GetMulti(keys)
	if err != nil {
		t.Error("must be no err")
	}
	for _, item := range items {
		if item.Key == "exists" {
			if string(item.Value) != "exists2" {
				t.Error("no exists2")
			}
		}
	}
	//close
	//err = b52.Close()
	//if err != nil {
	//t.Error(err)
	//}

}

/*
func Test_Pipeline(t *testing.T) {
	go main()
	time.Sleep(10 * time.Second)
	addr := ":11211"
	//pipeline

	c, err := net.Dial("tcp", addr)
	if err != nil {
		panic(err)
	}
	defer c.Close()

	rd := bufio.NewReader(c)
	w := bufio.NewWriter(c)

	fmt.Fprintf(w, "set c 0 0 1\r\n1\r\nset d 0 0 1\r\n2\r\n")
	err = w.Flush()
	if err != nil {
		panic(err)
	}
	p := make([]byte, 1024)
	n, err := rd.Read(p)
	if err != nil {
		panic(err)
	}

	if n != 16 {
		println(n, len(p))
		panic("err pipelining")
	}

	fmt.Fprintf(w, "get c\r\nget c\r\n")
	err = w.Flush()
	if err != nil {
		panic(err)
	}
	p = make([]byte, 1024)
	n, err = rd.Read(p)
	if err != nil {
		panic(err)
	}
	println(n, string(p))
	fmt.Fprintf(w, "get c\r\n")
	err = w.Flush()
	if err != nil {
		panic(err)
	}
	p = make([]byte, 1024)
	n, err = rd.Read(p)
	if err != nil {
		panic(err)
	}
	println(n, string(p))
}
*/
