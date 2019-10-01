package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/recoilme/mcproto"
)

var (
	listenaddr = flag.String("l", "", "Interface to listen on. Default to all addresses.")
	network    = flag.String("n", "tcp", "Network to listen on (tcp,tcp4,tcp6,unix). unix not tested! Default is tcp")
	port       = flag.Int("p", 11211, "TCP port number to listen on (default: 11211)")
	params     = flag.String("params", "", "params for engine, url query format:a=b&c=d")
)

func main() {
	flag.Parse()

	var b52 mcproto.McEngine
	b52, err := newb52()
	if err != nil {
		log.Fatalf("failed to create database: %s", err.Error())
	}

	address := fmt.Sprintf("%s:%d", *listenaddr, *port)

	listener, err := net.Listen(*network, address)
	if err != nil {
		log.Fatalf("failed to serve: %s", err.Error())
		return
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
		//return
		//b52.Close()
		os.Exit(1)
	}()
	// start service
	defer listener.Close()
	fmt.Printf("\nServer is listening on %s %s \n", *network, address)

	for {

		conn, err := listener.Accept()

		if err != nil {
			fmt.Println("conn", err)
			conn.Close()
			continue
		}
		go mcproto.ParseMc(conn, b52, "")
	}
}
