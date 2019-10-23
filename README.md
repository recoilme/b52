

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

This will retrieve and build the server.

## Starting

Use `./b52 --help` for full list of params. Example:

```sh
./b52 -p 11212 -params "sizelru=10&sizettl=10&dbdir="
# Start server on port 11212 with 10Mb lru and ttl cache size, without persistent database.
```

or just ./b52 - for start with default params

## How it work

`b52` is a layered database composed of a sniper, ristretto and freecache.
When b52 prepared properly, the ingredients separate into three distinctly visible layers.

It use [evio](https://github.com/tidwall/evio) for network communications.

[sniper](https://github.com/recoilme/sniper) - [fast](https://github.com/recoilme/sniper#performance), persistant on disk storage

[freecache](https://github.com/coocood/freecache) - in memory, [with zero GC overhead](https://github.com/coocood/freecache#features) cache, for keys with TTL (time to live) and LRU cache


### The balance between speed and efficiency is achieved as follows:

New entries go to disk (sniper). As you access them, they are cached in the LRU-cache (ristretto). Life-limited records are stored separately, in freecache (without persistance storage).

## Memory usage

For minimizing GC and allocations overhead - Sniper stored keys, and value addres/size in plain hash (map[uint32]uint32). HashMaps are fast, but has a memory cost. You must have 2Gb+ memory for storing every 100_000_000 entrys.

In Freecache and Ristretto memory is preallocated and it's size depends from you. 

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

## mc-benchmark

[mc-benchmark](https://github.com/antirez/mc-benchmark)
```
test #1 (default params)

./b52 -p 11222 -loops 10
sizelru: 100 Mb
sizettl: 100 Mb
dbdir: db
b52 server started on port 11222 (loops: 10)

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

test #2 (without ssd)

./b52 -p 11222 -loops 10 -params "sizelru=1024&sizettl=1024&dbdir="
./mc-benchmark -p 11222

====== SET ======
  10014 requests completed in 0.08 seconds
  50 parallel clients
  3 bytes payload
  keep alive: 1

60.14% <= 0 milliseconds
100.00% <= 1 milliseconds
122121.95 requests per second

====== GET ======
  10011 requests completed in 0.07 seconds
  50 parallel clients
  3 bytes payload
  keep alive: 1

63.75% <= 0 milliseconds
100.00% <= 1 milliseconds
135283.78 requests per second


test #3 with active slave

====== SET ======
  10000 requests completed in 0.14 seconds
  50 parallel clients
  3 bytes payload
  keep alive: 1

45.30% <= 0 milliseconds
85.45% <= 1 milliseconds
99.49% <= 2 milliseconds
100.00% <= 3 milliseconds
70422.53 requests per second

====== GET ======
  10014 requests completed in 0.08 seconds
  50 parallel clients
  3 bytes payload
  keep alive: 1

61.09% <= 0 milliseconds
100.00% <= 1 milliseconds
125175.00 requests per second

test #4  okdbc - not b52 test! it's my 1st database - [okdbc](https://github.com/recoilme/okdbc)

./mc-benchmark -p 11212
====== SET ======
  10001 requests completed in 0.75 seconds
  50 parallel clients
  3 bytes payload
  keep alive: 1

2.97% <= 0 milliseconds
60.16% <= 1 milliseconds
67.29% <= 2 milliseconds
71.61% <= 3 milliseconds
74.70% <= 4 milliseconds
78.20% <= 5 milliseconds
81.13% <= 6 milliseconds
84.43% <= 7 milliseconds
87.23% <= 8 milliseconds
88.69% <= 9 milliseconds
89.86% <= 10 milliseconds
91.35% <= 11 milliseconds
93.65% <= 12 milliseconds
94.46% <= 13 milliseconds
94.98% <= 14 milliseconds
95.86% <= 15 milliseconds
96.61% <= 16 milliseconds
97.00% <= 17 milliseconds
97.24% <= 18 milliseconds
97.34% <= 19 milliseconds
97.46% <= 20 milliseconds
98.12% <= 21 milliseconds
98.49% <= 22 milliseconds
98.86% <= 23 milliseconds
99.34% <= 24 milliseconds
99.40% <= 25 milliseconds
99.46% <= 26 milliseconds
99.52% <= 27 milliseconds
99.64% <= 28 milliseconds
99.79% <= 29 milliseconds
99.85% <= 30 milliseconds
99.89% <= 31 milliseconds
99.92% <= 32 milliseconds
99.93% <= 33 milliseconds
99.94% <= 34 milliseconds
99.96% <= 35 milliseconds
100.00% <= 36 milliseconds
13334.67 requests per second

====== GET ======
  10000 requests completed in 0.11 seconds
  50 parallel clients
  3 bytes payload
  keep alive: 1

47.28% <= 0 milliseconds
99.75% <= 1 milliseconds
100.00% <= 2 milliseconds
92592.59 requests per second
```


## Contact

Vadim Kulibaba [@recoilme](https://github.com/recoilme)

## License

`b52` source code is available under the MIT [License](/LICENSE).

