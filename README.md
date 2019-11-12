

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

Database params (running as master/slave, 1k-3k active connections, 100Gb lru cache,100 Mb ttl cache):
stats
STAT version 0.1.3
bytes 1814112504
heap_sys_mb 1656
curr_items 6074656
cmd_get 427387002
cmd_set 415174979
file_size 3774895840

test:

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


Or grab compiled binary version.
```



## Contact

Vadim Kulibaba [@recoilme](https://github.com/recoilme)

## License

`b52` source code is available under the MIT [License](/LICENSE).

