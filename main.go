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
	//network    = flag.String("n", "tcp", "Network to listen on (tcp,tcp4,tcp6,unix). unix not tested! Default is tcp")
	laddr    = flag.String("l", "", "Interface to listen on. Default to all addresses.")
	port     = flag.Int("p", 11211, "TCP port number to listen on (default: 11211)")
	slaveadr = flag.String("slave", "", "Slave address, optional, example slave=127.0.0.1:11212")
	params   = flag.String("params", "", "params for b52 engines, url query format, all size in Mb, default: sizelru=100&sizettl=100&dbdir=db")
)

func main() {
	flag.Parse()

	address := fmt.Sprintf("%s:%d", *laddr, *port)
	network := "tcp"
	listener, err := net.Listen(network, address)
	if err != nil {
		log.Fatalf("failed to serve: %s", err.Error())
		return
	}

	var b52 mcproto.McEngine
	b52, err = Newb52(*params, *slaveadr)
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
			fmt.Println("Close", err)
		}
		os.Exit(1)
	}()
	// start service
	defer listener.Close()
	fmt.Printf("\nServer is listening on %s %s \n", network, address)
	serve(listener, b52, "buf=4096&deadline=6000")
}

func serve(listener net.Listener, b52 mcproto.McEngine, mcparams string) {
	go func() {
		for {

			conn, err := listener.Accept()

			if err != nil {
				//fmt.Println("conn", err)
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
