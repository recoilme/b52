

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

`b52` contains from 3 layers.

[sniper](https://github.com/recoilme/sniper) - fast, persistant on disk storage

[ristretto](https://github.com/dgraph-io/ristretto) - effective, inmemory LRU cache for "hot" values

[freecache](https://github.com/coocood/freecache) - in memory cache, for keys with TTL (time to live)


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