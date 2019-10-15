package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
)

var (
	cmdSet     = []byte("set")
	cmdSetB    = []byte("SET")
	cmdGet     = []byte("get")
	cmdGetB    = []byte("GET")
	cmdGets    = []byte("gets")
	cmdGetsB   = []byte("GETS")
	cmdClose   = []byte("close")
	cmdCloseB  = []byte("CLOSE")
	cmdDelete  = []byte("delete")
	cmdDeleteB = []byte("DELETE")
	cmdIncr    = []byte("incr")
	cmdIncrB   = []byte("INCR")
	cmdDecr    = []byte("decr")
	cmdDecrB   = []byte("DECR")

	crlf     = []byte("\r\n")
	space    = []byte(" ")
	resultOK = []byte("OK\r\n")

	resultStored            = []byte("STORED\r\n")
	resultNotStored         = []byte("NOT_STORED\r\n")
	resultExists            = []byte("EXISTS\r\n")
	resultNotFound          = []byte("NOT_FOUND\r\n")
	resultDeleted           = []byte("DELETED\r\n")
	resultEnd               = []byte("END\r\n")
	resultOk                = []byte("OK\r\n")
	resultError             = []byte("ERROR\r\n")
	resultTouched           = []byte("TOUCHED\r\n")
	resultClientErrorPrefix = []byte("CLIENT_ERROR ")
)

var (
	//ErrClose dummy err for closing connection
	ErrClose = errors.New("memcache: close")
)

func parsemc(b []byte, db McEngine) ([]byte, []byte, error) {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		if i == 0 {
			//if start from \n - read \n
			return b[i+1:], nil, nil
		}
		line := b[:i+1]
		//print(string(line))

		switch {
		case bytes.HasPrefix(line, cmdClose), bytes.HasPrefix(line, cmdCloseB):
			//close
			return b, nil, ErrClose
		case bytes.HasPrefix(line, cmdSet), bytes.HasPrefix(line, cmdSetB):
			key, flags, exp, size, noreply, err := scanSetLine(line, bytes.HasPrefix(line, cmdSetB))
			if err != nil {
				return b, nil, err
			}
			//all commands in memcache - splited by line, except set, so handle it
			mustlen := i + 1 + size + 2 // pos(\n)+size+\r\n
			if len(b) < mustlen {
				//incomplete set, wait for all data
				//println("incomplete set", len(b), mustlen)
				return b, nil, nil
			}

			_, err = db.Set([]byte(key), b[i+1:i+1+size], flags, exp, size, noreply, nil)
			//println("set", string(key), size, string(b[i+1:i+1+size]), len(string(b[i+1:i+1+size])), err)
			if err != nil {
				return b[mustlen:], resultNotStored, err
			}
			return b[mustlen:], resultStored, err
		case bytes.HasPrefix(line, cmdGet), bytes.HasPrefix(line, cmdGetB), bytes.HasPrefix(line, cmdGets), bytes.HasPrefix(line, cmdGetsB):
			//println("get")
			cntspace := bytes.Count(line, space)
			if cntspace == 0 {
				return b, nil, errors.New("mailformed get request, no spaces")
			}
			if cntspace == 1 {
				key := line[(bytes.Index(line, space) + 1) : len(line)-2]
				//log.Println("'" + string(key) + "'")
				value, noreply, err := db.Get(key, nil)
				buf := bytes.NewBuffer([]byte{})
				if !noreply && err == nil && value != nil {
					//response:=new bytes.Buffer()
					fmt.Fprintf(buf, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
				}
				if !noreply {
					_, err = buf.Write(resultEnd)
				}
				return b[i+1:], buf.Bytes(), nil
			}
			//multiline get/gets
			args := bytes.Split(line[:len(line)-2], space)
			//strings.Split(string(line), " ")

			buf := bytes.NewBuffer([]byte{})
			rw := bufio.NewWriter(buf)
			_, err := db.Gets(args[1:], rw)
			//if err != nil {
			return b[i+1:], buf.Bytes(), err

		}
		return b[i+1:], nil, nil
	}
	//no line in request - return
	return b, nil, nil
}

// scanSetLine populates it and returns the declared params of the item.
// It does not read the bytes of the item.
func scanSetLine(line []byte, isCap bool) (key string, flags uint32, exp int32, size int, noreply bool, err error) {
	//set := ""
	noreplys := ""
	noreply = false
	cmd := "set"
	if isCap {
		cmd = "SET"
	}
	pattern := cmd + " %s %d %d %d %s\r\n"
	dest := []interface{}{&key, &flags, &exp, &size, &noreplys}
	if bytes.Count(line, space) == 4 {
		pattern = cmd + " %s %d %d %d\r\n"
		dest = dest[:4]
	}
	if noreplys == "noreply" || noreplys == "NOREPLY" {
		noreply = true
	}
	n, err := fmt.Sscanf(string(line), pattern, dest...)
	if n != len(dest) {
		err = errors.New("wrong set params" + string(line))
	}
	return
}
