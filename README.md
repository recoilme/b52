

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
$ go build
```

This will retrieve and build the server.

## How it work

`b52` is a layered database composed of a sniper, ristretto and freecache. 
When b52 prepared properly, the ingredients separate into three distinctly visible layers.

[sniper](https://github.com/recoilme/sniper) - [fast](https://github.com/recoilme/sniper#performance), persistant on disk storage

[ristretto](https://github.com/dgraph-io/ristretto) - [effective](https://github.com/dgraph-io/ristretto#features), inmemory LRU cache for "hot" values

[freecache](https://github.com/coocood/freecache) - in memory, [with zero GC overhead](https://github.com/coocood/freecache#features) cache, for keys with TTL (time to live)


### The balance between speed and efficiency is achieved as follows:

New entries go to disk (sniper). As you access them, they are cached in the LRU-cache (ristretto). Life-limited records are stored separately, in freecache (without persistance storage).

## Memory usage

For minimizing GC and allocations overhead - Sniper stored keys, and value addres/size in plain hash (map[uint64]uint64). HashMaps are fast, but has a memory cost. You must have 4Gb+ memory for storing every 100_000_000 entrys.

In Freecache and Ristretto memory is preallocated and it's size depends from you. 

## Disk usage

Sniper has a minimum 8 byte overhead on every entry. But it allocate space in power of 2, and try to reuse space, if value grow. And sniper will try to reuse space from deleted/evicted records.

## Usage (telnet example)

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


## Contact

Vadim Kulibaba [@recoilme](https://github.com/recoilme)

## License

`b52` source code is available under the MIT [License](/LICENSE).