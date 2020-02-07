

<p align="center">
<img 
    src="https://upload.wikimedia.org/wikipedia/commons/3/3a/Cocktail_B52.jpg" 
    width="128" height="200" border="0" alt="b-52">
    <br>
    b52
</p>


[![GoDoc](https://img.shields.io/badge/api-reference-blue.svg?style=flat-square)](https://godoc.org/github.com/recoilme/b52)

b52 is a fast experimental Key/value database. With support for the memcache protocol.


# Getting Started

## Installing

To start using `b52`, install Go and run `go get`:

```sh
$ go get -u github.com/recoilme/b52
$ go build or go install
```

This will retrieve and build the server. Or grab compiled binary version.

## Starting

Use `./b52 --help` for full list of params. Example:

```sh
./b52 -p 11212 -params "sizelru=10&sizettl=10&dbdir="
# Start server on port 11212 with 10Mb lru and ttl cache size, without persistent database.
```

or just ./b52 - for start with default params

## How it work

`b52` is a layered database composed of a sniper, evio and freecache.
When b52 prepared properly, the ingredients separate into three distinctly visible layers.

It use [evio](https://github.com/tidwall/evio) for network communications.

[sniper](https://github.com/recoilme/sniper) - [fast](https://github.com/recoilme/sniper#performance), persistant on disk storage

[freecache](https://github.com/coocood/freecache) - in memory, [with zero GC overhead](https://github.com/coocood/freecache#features) cache, for keys with TTL (time to live) and LRU cache


### The balance between speed and efficiency is achieved as follows:

New entries go to disk (sniper). As you access them, they are cached in the LRU-cache (freecache). Life-limited records are stored separately, in freecache (without persistance storage).

## Memory usage

For minimizing GC and allocations overhead - Sniper stored keys, and value addres/size in plain hash (map[uint32]uint32). HashMaps are fast, but has a memory cost. You must have 2Gb+ memory for storing every 100_000_000 entrys.

In Freecache memory is preallocated and it's size depends from you. 

## Disk usage

Sniper has a minimum 8 byte overhead on every entry. But it allocate space in power of 2, and try to reuse space, if value grow. Also, sniper will try to reuse space from deleted/evicted records.

## Telnet example

```
telnet localhost 11211
set a 0 0 5
12345
STORED
get a
VALUE a 0 5
12345
END
close
```

## Memcache protocol

b52 use text version of [memcache protocol](https://github.com/memcached/memcached/blob/master/doc/protocol.txt). With this commands:
```
	cmdAdd       = []byte("add")
	cmdReplace   = []byte("replace")
	cmdSet       = []byte("set")
	cmdGet       = []byte("get")
	cmdGets      = []byte("gets")
	cmdClose     = []byte("close")
	cmdDelete    = []byte("delete")
	cmdIncr      = []byte("incr")
	cmdDecr      = []byte("decr")
	cmdStats     = []byte("stats")
	cmdQuit      = []byte("quit")
	cmdVersion   = []byte("version")
```

## mc-benchmark

[mc-benchmark](https://github.com/antirez/mc-benchmark)

```
Database params (running as master/slave, 1Gb lru cache, 100 Mb ttl cache, stats after 3 days using):
stats
STAT version 0.1.3
bytes 2301825272
heap_sys_mb 2103
curr_items 17165174
cmd_get 6660544669
cmd_set 6494641380
file_size 14172392544

test (on production, with ~2k active connections at same time):
./mc-benchmark -p 11222

====== SET ======
  10000 requests completed in 0.10 seconds
  50 parallel clients
  3 bytes payload
  keep alive: 1

56.39% <= 0 milliseconds
95.39% <= 1 milliseconds
100.00% <= 2 milliseconds
101010.10 requests per second

====== GET ======
  10014 requests completed in 0.08 seconds
  50 parallel clients
  3 bytes payload
  keep alive: 1

60.25% <= 0 milliseconds
100.00% <= 1 milliseconds
123629.63 requests per second

./mc-benchmark -n 100000 -p 11212
====== SET ======
  100003 requests completed in 1.03 seconds
  50 parallel clients
  3 bytes payload
  keep alive: 1

49.59% <= 0 milliseconds
99.16% <= 1 milliseconds
100.00% <= 2 milliseconds
96996.12 requests per second

====== GET ======
  100018 requests completed in 0.98 seconds
  50 parallel clients
  3 bytes payload
  keep alive: 1

51.55% <= 0 milliseconds
99.72% <= 1 milliseconds
99.99% <= 2 milliseconds
100.00% <= 3 milliseconds
101851.33 requests per second
```

[expvar](https://gist.github.com/recoilme/0624cd5ecda195c804b67b1d64394603)

## Contact

Vadim Kulibaba [@recoilme](https://github.com/recoilme)

## License

`b52` source code is available under the MIT [License](/LICENSE).

