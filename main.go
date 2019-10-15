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
	//network    = flag.String("n", "tcp", "Network to listen on (tcp,tcp4,tcp6,unix). unix not tested! Default is tcp")
	laddr    = flag.String("l", "", "Interface to listen on. Default to all addresses.")
	port     = flag.Int("p", 11211, "TCP port number to listen on (default: 11211)")
	slaveadr = flag.String("slave", "", "Slave address, optional, example slave=127.0.0.1:11212")
	params   = flag.String("params", "", "params for b52 engines, url query format, all size in Mb, default: sizelru=100&sizettl=100&dbdir=db")
)

type conn struct {
	is   evio.InputStream
	addr string
}

func main() {
	flag.Parse()
	//port := 11211

	//address := fmt.Sprintf("%s:%d", *laddr, *port)
	//network := "tcp"
	//listener, err := net.Listen(network, address)
	//if err != nil {
	//	log.Fatalf("failed to serve: %s", err.Error())
	//	return
	//}

	var b52 McEngine
	b52, err := Newb52(*params, *slaveadr)
	if err != nil {
		log.Fatalf("failed to create database: %s", err.Error())
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
		if q == syscall.SIGPIPE || q.String() == "broken pipe" {
			return
		}
		err = b52.Close()
		if err != nil {
			fmt.Println("Close", err.Error())
		}
		os.Exit(1)
	}()
	// start service
	//defer listener.Close()
	//fmt.Printf("\nServer is listening on %s %s \n", network, address)
	//serve(listener, b52, "buf=81920&deadline=1200000")
	//select {}
	var events evio.Events
	balance := "random"
	switch balance {
	default:
		log.Fatalf("invalid -balance flag: '%v'", balance)
	case "random":
		events.LoadBalance = evio.Random
	case "round-robin":
		events.LoadBalance = evio.RoundRobin
	case "least-connections":
		events.LoadBalance = evio.LeastConnections
	}
	loops := -1
	events.NumLoops = loops
	events.Serving = func(srv evio.Server) (action evio.Action) {
		fmt.Printf("server started on port %d (loops: %d)\n", *port, srv.NumLoops)

		return
	}
	events.Opened = func(ec evio.Conn) (out []byte, opts evio.Options, action evio.Action) {
		//fmt.Printf("opened: %v\n", ec.RemoteAddr())
		opts.TCPKeepAlive = time.Minute * 25
		ec.SetContext(&conn{})
		return
	}
	events.Closed = func(ec evio.Conn, err error) (action evio.Action) {
		//fmt.Printf("closed: %v\n", ec.RemoteAddr())
		return
	}

	events.Data = func(ec evio.Conn, in []byte) (out []byte, action evio.Action) {
		if in == nil {
			log.Printf("wake from %s\n", ec.RemoteAddr())
			return nil, evio.Close
		}
		c := ec.Context().(*conn)
		data := c.is.Begin(in)
		for {
			leftover, response, err := parsemc(data, b52)
			if err != nil {
				if err != ErrClose {
					// bad thing happened
					println(err.Error())
					//out = appendresp(out, "500 Error", "", err.Error()+"\n")
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
	//if stdlib {
	//	ssuf = "-net"
	//}
	addrs := []string{fmt.Sprintf("tcp"+ssuf+"://:%d", *port)}
	//if unixsocket != "" {
	//addrs = append(addrs, fmt.Sprintf("unix"+ssuf+"://%s", unixsocket))
	//}
	err = evio.Serve(events, addrs...)
	if err != nil {
		log.Fatal(err)
	}
}

/*
func serve(listener net.Listener, b52 mcproto.McEngine, mcparams string) {
	go func() {
		for {

			conn, err := listener.Accept()

			if err != nil {
				fmt.Println("conn", err.Error())
				if conn != nil {
					conn.Close()
				}
				continue
			} else {
				//println("Accept")
				go mcproto.ParseMc(conn, b52, mcparams)
			}

		}
	}()
}
*/
