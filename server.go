package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/golang/snappy"
	"github.com/spaolacci/murmur3"
)

// Server is a HTTP interface
type Server struct {
	Db McEngine
}

// Cluster stored in db
type Cluster struct {
	ID      string
	Domains []string
}

var (
	warm uint32
	cold uint32
	zero uint32
	errs uint32
)

func init() {
	atomic.StoreUint32(&warm, 0)
	atomic.StoreUint32(&cold, 0)
	atomic.StoreUint32(&zero, 0)
	atomic.StoreUint32(&errs, 0)
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/")
	switch r.Method {
	case http.MethodPost:

	case http.MethodGet:
		switch {
		case path == "":
			date := time.Now().UTC().Format(http.TimeFormat)
			w.Header().Set("Date", date)
			w.Header().Set("Server", "b52 "+version)
			w.Header().Set("Connection", "Closed")
			fmt.Fprint(w, "status = \"OK\"\r\n")
		case strings.HasPrefix(path, "pulse:2:"):
			s.api(path, w)
			return
		default:
			w.WriteHeader(404)
		}
	}
}

func (s *Server) api(key string, w http.ResponseWriter) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if strings.HasPrefix(key, "pulse:2:") {
		value, err := pulse(s.Db, []byte(key))
		if murmur3.Sum32WithSeed([]byte(key), 0)%10 == 0 && len(value) > 4 {
			llog("key:", key, string(value[:4]), len(value), "errs:", atomic.LoadUint32(&errs),
				"\nzero:", atomic.LoadUint32(&zero), "cold:", atomic.LoadUint32(&cold), "warm:", atomic.LoadUint32(&warm))
		}

		if err != nil {
			fmt.Println(err, key)
			atomic.AddUint32(&errs, 1)
			w.WriteHeader(500)
			return
		} //pulse:2:2562814585298765628
		fmt.Fprint(w, string(value))
		return
	}

	value, err := s.Db.Get([]byte(key))
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(500)
		return
	}
	fmt.Fprint(w, string(value))
}

func pulse(db McEngine, key []byte) (value []byte, err error) {
	digit := []byte(",")
	// clusters
	ssd := db.Store()
	value, err = ssd.Get([]byte("clusters"))
	if err != nil {
		fmt.Println("no clust")
		return
	}
	value, err = snappy.Decode(nil, value)
	if err != nil {
		return
	}
	var clusters []Cluster
	err = json.Unmarshal(value, &clusters)
	if err != nil {
		return
	}
	if len(clusters) == 0 {
		return nil, errors.New("No clusters")
	}
	//map [cluster] recs
	recsCl := make(map[string][]string)
	for _, cluster := range clusters {
		clusterkey := fmt.Sprintf("cluster:%d:%s:%s:0", 6, "pulse.mail.ru", cluster.ID)
		if recs, err := db.Get([]byte(clusterkey)); err == nil {
			splited := strings.Split(string(recs), ",")
			var recsstr []string
			for i, val := range splited {
				if i%2 == 0 {
					if val != "" {
						recsstr = append(recsstr, val)
					}
				}
			}

			recsCl[cluster.ID] = recsstr
		}
	}

	// randomize itterator
	seed := int64(time.Now().UnixNano())
	rnd := rand.New(rand.NewSource(seed))
	ints := rnd.Perm(len(clusters))
	var merged []string
	stop := false
	for i := 0; i < 300; i++ {
		if stop {
			break
		}
		for _, randIdx := range ints {
			// рандомный кластер
			if recs, ok := recsCl[clusters[randIdx].ID]; ok {
				if len(recs) > i {
					merged = append(merged, recs[i])
					if len(merged) >= 400 {
						stop = true
						break
					}
				}
			}
		}
	}
	// check personal
	key = bytes.TrimPrefix(key, []byte("pulse:2:"))

	clickskey := fmt.Sprintf("recs:%d:%s:0:%s", 1, "pulse.mail.ru", string(key))

	var recsPersonal []string
	var clicks []byte

	if string(key) == "0" {
		atomic.AddUint32(&zero, 1)
	} else {
		clicks, err = db.Get([]byte(clickskey))
	}
	if err == nil && string(key) != "0" {
		splited := bytes.Split(clicks, digit)
		for i, url := range splited {
			if i%2 == 0 {
				key := fmt.Sprintf("recs:%d:%s:0:%s", 7888, "pulse.mail.ru", string(url))
				if err != nil {
					continue
				}
				recsstr, err := db.Get([]byte(key))
				if err != nil {
					continue
				}
				spl := bytes.Split(recsstr, digit)
				for j, rec := range spl {
					if j%2 == 0 {
						recsPersonal = append(recsPersonal, string(rec))
					}
				}
			}
		}
	}
	buf := bytes.Buffer{}
	if len(recsPersonal) > 0 {
		buf.WriteString("warm_user,100")
		if string(key) != "0" {
			atomic.AddUint32(&warm, 1)
		}
	} else {
		buf.WriteString("cold_user,100")
		if string(key) != "0" {
			atomic.AddUint32(&cold, 1)
		}
	}
	ints = rnd.Perm(len(recsPersonal))
	//fmt.Println("merged:", merged)
	for i, rec := range merged {
		if i%4 == 0 && len(recsPersonal) > 0 && len(recsPersonal) > i/4 {
			// insert random personal rec
			buf.WriteByte(byte(9)) //tab
			buf.Write(bytes.Replace([]byte(recsPersonal[ints[i/4]]), []byte("_"), digit, 1))
		}
		buf.WriteByte(byte(9)) //tab
		buf.Write(bytes.Replace([]byte(rec), []byte("_"), digit, 1))
	}
	value = buf.Bytes()
	if string(key) == "1212118267992805046" {
		fmt.Println("Kulibaba:\n", "recsPersonal:", recsPersonal, "\n merged:", string(value))
	}
	err = nil //key not found error
	return
}

func llog(a ...interface{}) (n int, err error) {
	buf := bytes.Buffer{}
	buf.WriteString(fmt.Sprintf("%s ", time.Now().Format("15:04:05")))
	for _, s := range a {
		buf.WriteString(fmt.Sprint(s, " "))
	}
	return fmt.Fprintln(os.Stdout, buf.String())
}
