package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tidwall/evio"
)

var (
	version   = "0.1.0"
	port      = flag.Int("p", 11211, "TCP port number to listen on (default: 11211)")
	slaveadr  = flag.String("slave", "", "Slave address, optional, example slave=127.0.0.1:11212")
	unixs     = flag.String("unixs", "", "unix socket")
	stdlib    = flag.Bool("stdlib", false, "use stdlib")
	loops     = flag.Int("loops", 0, "num loops")
	balance   = flag.String("balance", "random", "balance - random, round-robin or least-connections")
	keepalive = flag.Int("keepalive", 0, "keepalive connection, in seconds")
	params    = flag.String("params", "", "params for b52 engines, url query format, all size in Mb, default: sizelru=100&sizettl=100&dbdir=db")
)

type conn struct {
	is   evio.InputStream
	addr string
}

func main() {
	flag.Parse()

	var b52 McEngine
	b52, err := Newb52(*params, *slaveadr)
	if err != nil {
		log.Fatalf("failed to create Newb52 database: %s", err.Error())
	}

	// Wait for interrupt signal to gracefully shutdown the server with
	// setup signal catching
	quit := make(chan os.Signal, 1)
	// catch all signals since not explicitly listing
	signal.Notify(quit)
	// method invoked upon seeing signal
	go func() {
		q := <-quit
		fmt.Printf("\nRECEIVED SIGNAL: %s\n", q)
		//ignore broken pipe?
		if q == syscall.SIGPIPE || q.String() == "broken pipe" {
			//return
		}
		err = b52.Close()
		if err != nil {
			fmt.Println("Close", err.Error())
		}
		os.Exit(1)
	}()

	var events evio.Events
	switch *balance {
	default:
		log.Fatalf("invalid -balance flag: '%v'", balance)
	case "random":
		events.LoadBalance = evio.Random
	case "round-robin":
		events.LoadBalance = evio.RoundRobin
	case "least-connections":
		events.LoadBalance = evio.LeastConnections
	}

	events.NumLoops = *loops
	events.Serving = func(srv evio.Server) (action evio.Action) {
		fmt.Printf("b52 server started on port %d (loops: %d)\n", *port, srv.NumLoops)
		return
	}
	events.Opened = func(ec evio.Conn) (out []byte, opts evio.Options, action evio.Action) {
		//fmt.Printf("opened: %v\n", ec.RemoteAddr())
		if (*keepalive) > 0 {
			opts.TCPKeepAlive = time.Second * (time.Duration(*keepalive))
		}
		ec.SetContext(&conn{})
		return
	}
	events.Closed = func(ec evio.Conn, err error) (action evio.Action) {
		//fmt.Printf("closed: %v\n", ec.RemoteAddr())
		return
	}

	events.Data = func(ec evio.Conn, in []byte) (out []byte, action evio.Action) {
		if in == nil {
			fmt.Printf("wake from %s\n", ec.RemoteAddr())
			return nil, evio.Close
		}
		c := ec.Context().(*conn)
		data := c.is.Begin(in)
		for {
			leftover, response, err := mcproto(data, b52)
			if err != nil {
				if err != ErrClose {
					// bad thing happened
					println(err.Error())
				}
				action = evio.Close
				break
			} else if len(leftover) == len(data) {
				// request not ready, yet
				break
			}
			// handle the request
			out = response
			data = leftover
		}
		c.is.End(data)
		return
	}
	var ssuf string
	if *stdlib {
		ssuf = "-net"
	}
	addrs := []string{fmt.Sprintf("tcp"+ssuf+"://:%d", *port)} //?reuseport=true
	if *unixs != "" {
		addrs = append(addrs, fmt.Sprintf("unix"+ssuf+"://%s", *unixs))
	}
	err = evio.Serve(events, addrs...)
	if err != nil {
		println(err.Error())
		log.Fatal(err)
	}
}
