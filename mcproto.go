package main

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
)

var (
	cmdAdd       = []byte("add")
	cmdReplace   = []byte("replace")
	cmdSet       = []byte("set")
	cmdTouch     = []byte("touch")
	cmdGet       = []byte("get")
	cmdGets      = []byte("gets")
	cmdClose     = []byte("close")
	cmdCloseB    = []byte("CLOSE")
	cmdDelete    = []byte("delete")
	cmdIncr      = []byte("incr")
	cmdDecr      = []byte("decr")
	cmdStats     = []byte("stats")
	cmdQuit      = []byte("quit")
	cmdQuitB     = []byte("QUIT")
	cmdVersion   = []byte("version")
	cmdVerbosity = []byte("verbosity")
	cmdBackup    = []byte("backup")

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
	resultServerErrorPrefix = []byte("SERVER_ERROR ") //
)

var (
	//ErrClose dummy err for closing connection
	ErrClose = errors.New("memcache: close")
)

//https://github.com/memcached/memcached/blob/master/doc/protocol.txt
func mcproto(b []byte, db McEngine) ([]byte, []byte, error) {

	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		if i == 0 {
			//if start from \n - read \n
			//fmt.Println("start from \n - read n")
			return b[i+1:], nil, nil
		}
		line := b[:i+1]

		switch {
		default:
			//fmt.Println(string(line))
		case bytes.HasPrefix(line, cmdClose), bytes.HasPrefix(line, cmdCloseB), bytes.HasPrefix(line, cmdQuit), bytes.HasPrefix(line, cmdQuitB):
			//close
			//fmt.Println("close")
			return b, nil, ErrClose
		case bytes.HasPrefix(line, cmdAdd):
			//  "add" means "store this data, but only if the server *doesn't* already
			key, flags, exp, size, noreply, err := scanSetLine(line, string(cmdAdd))
			if err != nil {
				return b, nil, err
			}
			mustlen := i + 1 + size + 2 // pos(\n)+size+\r\n
			if len(b) < mustlen {
				//incomplete cmd, wait for all data
				return b, nil, nil
			}
			_, err = db.Get([]byte(key))
			if err != nil {
				if !strings.Contains(err.Error(), "not found") {
					return b[mustlen:], resultNotStored, err
				}
			} else {
				//has this key
				return b[mustlen:], resultNotStored, err
			}
			err = db.Set([]byte(key), b[i+1:i+1+size], flags, exp, size, noreply)
			if noreply {
				return b[mustlen:], nil, err
			}
			if err != nil {
				return b[mustlen:], resultNotStored, err
			}
			return b[mustlen:], resultStored, err

		case bytes.HasPrefix(line, cmdReplace):
			//  "replace" means "store this data, but only if the server *does*
			key, flags, exp, size, noreply, err := scanSetLine(line, string(cmdReplace))
			if err != nil {
				return b, nil, err
			}
			mustlen := i + 1 + size + 2 // pos(\n)+size+\r\n
			if len(b) < mustlen {
				//incomplete cmd, wait for all data
				return b, nil, nil
			}
			_, err = db.Get([]byte(key))
			if err != nil {
				return b[mustlen:], resultNotStored, nil
			}
			err = db.Set([]byte(key), b[i+1:i+1+size], flags, exp, size, noreply)
			if noreply {
				return b[mustlen:], nil, err
			}
			if err != nil {
				return b[mustlen:], resultNotStored, err
			}
			return b[mustlen:], resultStored, err

		case bytes.HasPrefix(line, cmdSet):
			key, flags, exp, size, noreply, err := scanSetLine(line, string(cmdSet))
			if err != nil {
				return b, nil, err
			}
			//all commands in memcache - splited by line, except set, so handle it
			mustlen := i + 1 + size + 2 // pos(\n)+size+\r\n
			if len(b) < mustlen {
				//incomplete set, wait for all data
				//fmt.Println("incomplete set", len(b), mustlen)
				return b, nil, nil
			}
			err = db.Set([]byte(key), b[i+1:i+1+size], flags, exp, size, noreply)
			//println("set", string(key), size, string(b[i+1:i+1+size]), len(string(b[i+1:i+1+size])), err)
			if noreply {
				//fmt.Println("noreply set")
				return b[mustlen:], nil, err
			}
			if err != nil {
				//fmt.Println(err.Error())
				return b[mustlen:], resultNotStored, err
			}
			return b[mustlen:], resultStored, err

		case bytes.HasPrefix(line, cmdTouch):
			key, exp, noreply, err := scanTouchLine(line, string(cmdTouch))
			if err != nil {
				return b, nil, err
			}
			err = db.Touch([]byte(key), exp)
			if noreply {
				//fmt.Println("noreply set")
				return b[i+1:], nil, err
			}
			if err != nil {
				if !strings.Contains(err.Error(), "not found") {
					return b[i+1:], resultNotStored, err
				}
				return b[i+1:], resultNotFound, nil
			}
			return b[i+1:], resultTouched, nil

		case bytes.HasPrefix(line, cmdGet):
			//fmt.Println("get")
			cntspace := bytes.Count(line, space)
			if cntspace == 0 {
				return b, nil, errors.New("mailformed get request, no spaces")
			}
			if cntspace == 1 {
				key := line[(bytes.Index(line, space) + 1) : len(line)-2]
				//log.Println("'" + string(key) + "'")
				value, err := db.Get(key)
				buf := bytes.NewBuffer([]byte{})
				if err == nil && value != nil {
					//response:=new bytes.Buffer()
					fmt.Fprintf(buf, "VALUE %s 0 %d\r\n%s\r\n", key, len(value), value)
				}
				buf.Write(resultEnd)
				return b[i+1:], buf.Bytes(), nil
			}
			//multiline get/gets
			args := bytes.Split(line[:len(line)-2], space)
			//fmt.Println("get args:", len(args))
			response, err := db.Gets(args[1:])
			//fmt.Println("get response:", string(response))
			if err != nil {
				fmt.Println("get err", err.Error())
			}

			return b[i+1:], response, err

		case bytes.HasPrefix(line, cmdStats):
			res, err := db.Stats()
			return b[i+1:], res, err
		case bytes.HasPrefix(line, cmdDelete):
			key, noreply, err := scanDeleteLine(line)
			if err != nil {
				return b, nil, err
			}
			deleted, _ := db.Delete([]byte(key))
			//if !noreply {
			_ = noreply
			if deleted {
				return b[i+1:], resultDeleted, nil
			}
			// err mean not deleted
			return b[i+1:], resultNotFound, nil
			//}

		case bytes.HasPrefix(line, cmdBackup):
			filename, _, err := scanBackupLine(line)
			if err != nil {
				return b, nil, err
			}
			err = db.Backup(filename)
			if err == nil {
				return b[i+1:], resultStored, nil
			}
			// err mean not deleted
			return b[i+1:], nil, err

		case bytes.HasPrefix(line, cmdIncr):
			return b[i+1:], resultError, nil
		case bytes.HasPrefix(line, cmdVersion):
			return b[i+1:], []byte("VERSION " + version + "\r\n"), nil
		case bytes.HasPrefix(line, cmdDecr):
			return b[i+1:], resultError, nil
		case bytes.HasPrefix(line, cmdVerbosity):
			return b[i+1:], resultOK, nil
		}
		return b[i+1:], nil, nil
	}

	return b, nil, nil
}

// scanSetLine populates it and returns the declared params of the item.
// It does not read the bytes of the item.
//<command name> <key> <flags> <exptime> <bytes> [noreply]\r\n
//- <command name> is "set", "add", "replace"
//  "set" means "store this data"
//  "add" means "store this data, but only if the server *doesn't* already
//  hold data for this key".
//  "replace" means "store this data, but only if the server *does*
//  already hold data for this key".
func scanSetLine(line []byte, cmd string) (key string, flags uint32, exp uint32, size int, noreply bool, err error) {
	//set := ""
	noreplys := ""
	noreply = false
	pattern := cmd + " %s %d %d %d %s\r\n"
	dest := []interface{}{&key, &flags, &exp, &size, &noreplys}

	if bytes.Count(line, space) == 4 {
		pattern = cmd + " %s %d %d %d\r\n"
		dest = dest[:4]
	}
	n, err := fmt.Sscanf(string(line), pattern, dest...)
	if noreplys == "noreply" || noreplys == "NOREPLY" {
		noreply = true
	}
	if n != len(dest) {
		err = errors.New("wrong set params" + string(line))
	}
	return
}

// scanTouchLine populates it and returns the declared params of the item.
// It does not read the bytes of the item.
// touch <key> <exptime> [noreply]\r\n
func scanTouchLine(line []byte, cmd string) (key string, exp uint32, noreply bool, err error) {
	//set := ""
	noreplys := ""
	noreply = false
	pattern := cmd + " %s %d %s\r\n"
	dest := []interface{}{&key, &exp, &noreplys}

	if bytes.Count(line, space) == 2 {
		pattern = cmd + " %s %d\r\n"
		dest = dest[:2]
	}
	n, err := fmt.Sscanf(string(line), pattern, dest...)
	if noreplys == "noreply" || noreplys == "NOREPLY" {
		noreply = true
	}
	if n != len(dest) {
		err = errors.New("wrong touch params" + string(line))
	}
	return
}

// scanDeleteLine populates it and returns the declared params of the item.
// It does not read the bytes of the item.
func scanDeleteLine(line []byte) (key string, noreply bool, err error) {
	//set := ""
	noreplys := ""
	noreply = false
	cmd := "delete"

	pattern := cmd + " %s %s\r\n"
	dest := []interface{}{&key, &noreplys}
	if bytes.Count(line, space) == 1 {
		pattern = cmd + " %s\r\n"
		dest = dest[:1]
	}
	n, err := fmt.Sscanf(string(line), pattern, dest...)
	if noreplys == "noreply" || noreplys == "NOREPLY" {
		noreply = true
	}
	if n != len(dest) {
		err = errors.New(string(resultError))
	}
	return
}

// scanIncrDecrLine populates it and returns the declared params of the item.
// It does not read the bytes of the item.
func scanIncrDecrLine(line []byte, incr bool, isCap bool) (key string, val uint64, noreply bool, err error) {
	//set := ""
	noreplys := ""
	noreply = false
	cmd := "incr"
	if !incr {
		cmd = "decr"
	}
	if isCap {
		cmd = "INCR"
		if !incr {
			cmd = "DECR"
		}
	}

	pattern := cmd + " %s %d %s\r\n"
	dest := []interface{}{&key, &val, &noreplys}
	if bytes.Count(line, space) == 2 {
		pattern = cmd + " %s %d\r\n"
		dest = dest[:2]
	}

	n, err := fmt.Sscanf(string(line), pattern, dest...)
	if noreplys == "noreply" || noreplys == "NOREPLY" {
		noreply = true
	}
	if n != len(dest) {
		err = errors.New(string(resultError))
	}
	return
}

// scanBackupLine populates it and returns the backup filename
func scanBackupLine(line []byte) (filename string, noreply bool, err error) {
	noreplys := ""
	noreply = false
	cmd := "backup"

	pattern := cmd + " %s %s\r\n"
	dest := []interface{}{&filename, &noreplys}
	if bytes.Count(line, space) == 1 {
		pattern = cmd + " %s\r\n"
		dest = dest[:1]
	}
	n, err := fmt.Sscanf(string(line), pattern, dest...)
	if noreplys == "noreply" || noreplys == "NOREPLY" {
		noreply = true
	}
	if n != len(dest) {
		err = errors.New(string(resultError))
	}
	return
}
