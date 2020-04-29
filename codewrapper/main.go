package main

import (
	"fmt"
	"net"

	"github.com/Syncano/codebox/codewrapper/server"
)

const (
	port = 8123
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func getIPAddress() (ip net.IP) {
	// Get address for eth0.
	i, err := net.InterfaceByName("eth0")
	must(err)
	addrs, err := i.Addrs()
	must(err)

	for _, addr := range addrs {
		if v, ok := addr.(*net.IPNet); ok {
			ip = v.IP
			break
		}
	}

	return
}

func main() {
	ip := getIPAddress()
	addr := fmt.Sprintf("%s:%d", ip.String(), port)

	s, err := server.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}

	err = s.StartAccepting()
	if err != nil {
		panic(err)
	}
}
