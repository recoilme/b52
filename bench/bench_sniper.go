package main

import (
	"fmt"
	"math/rand"
	"os"
	"runtime"
	"time"

	"encoding/json"

	"hash/fnv"

	"github.com/recoilme/sniper"
	"github.com/tidwall/lotsa"
)

var h = fnv.New32a()

func hash(s string) uint32 {
	MASK_24 := uint32((1 << 24) - 1)
	h.Reset()
	h.Write([]byte(s))
	ss := h.Sum32()
	hash := (ss >> 24) ^ (ss & MASK_24)
	return hash
}

type userWithMeta struct {
	UserId   string
	Segments []int
	Time     uint32
}

func main() {
	//TestSniperSpeed()
	//map32()
	fmt.Println(hash("HelloWorld"))
	fmt.Println(hash("HelloWorld."))
}

func map32() {

	//map[uint64]uint64 - 4158 MiB Total = 5674 MiB
	//map[uint32]uint32 - 2363 MiB Total = 3260 MiB
	//map[uint64]uint8  - 2608 MiB Total = 3592 MiB
	m := make(map[uint64]uint64)
	for i := 0; i < 100_000_000; i++ {
		ui32 := uint64(i)
		m[ui32] = ui32
	}
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	fmt.Printf("Alloc = %v MiB Total = %v MiB\n", bToMb(ms.Alloc), bToMb(ms.TotalAlloc))
}

func bToMb(b uint64) uint64 {
	return b / 1024 / 1024
}

func randKey(rnd *rand.Rand, n int) []byte {
	s := make([]byte, n)
	rnd.Read(s)
	for i := 0; i < n; i++ {
		s[i] = 'a' + (s[i] % 26)
	}
	return s
}

func TestSniperSpeed() {

	seed := time.Now().UnixNano()
	// println(seed)
	rng := rand.New(rand.NewSource(seed))
	N := 10_000_000
	K := 10

	fmt.Printf("\n")
	fmt.Printf("go version %s %s/%s\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
	fmt.Printf("\n")
	fmt.Printf("     number of cpus: %d\n", runtime.NumCPU())
	fmt.Printf("     number of keys: %d\n", N)
	fmt.Printf("            keysize: %d\n", K)
	fmt.Printf("        random seed: %d\n", seed)

	fmt.Printf("\n")

	keysm := make(map[string]string, N)
	iter := 0
	for len(keysm) < N {
		id := string(randKey(rng, K))
		u := userWithMeta{}
		u.UserId = id
		u.Segments = make([]int, 2)
		u.Segments[0] = iter
		u.Segments[1] = iter + 1
		u.Time = uint32(time.Now().Unix())
		bin, _ := json.Marshal(&u)
		//println(err, string(bin))
		keysm[id] = string(bin)
	}
	keys := make([][]byte, 0, N)
	for key := range keysm {
		keys = append(keys, []byte(key))
	}

	lotsa.Output = os.Stdout
	lotsa.MemUsage = true

	println("-- sniper --")
	sniper.DeleteStore("1")
	s, _ := sniper.Open("1")
	print("set: ")
	lotsa.Ops(N, runtime.NumCPU(), func(i, _ int) {
		s.Set(keys[i], []byte(keysm[string(keys[i])]))
	})
	print("get: ")
	lotsa.Ops(N, runtime.NumCPU(), func(i, _ int) {
		b, err := s.Get(keys[i])
		if err != nil {
			panic(err)
		}
		if i%300_000 == 0 {
			_ = b
			//println()
			//println(string(b))
			//println()
		}

	})

	print("del: ")
	lotsa.Ops(N, runtime.NumCPU(), func(i, _ int) {
		s.Delete(keys[i])
	})
	sniper.DeleteStore("1")
	println()
}
