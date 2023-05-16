package main

import (
	"net"
	"syscall"
)

func main() {
	// Set the destination address and port
	destAddr := net.UDPAddr{
		IP:   net.ParseIP("192.122.190.167"), // replace with your destination IP
		Port: 62946,                      // replace with your destination port
	}

	// Create a UDP connection
	conn, err := net.DialUDP("udp", nil, &destAddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	// Get the file descriptor
	fd, err := conn.File()
	if err != nil {
		panic(err)
	}
	defer fd.Close()

	// Set the TTL
	err = syscall.SetsockoptInt(int(fd.Fd()), syscall.IPPROTO_IP, syscall.IP_TTL, 3) // replace 64 with your desired TTL
	if err != nil {
		panic(err)
	}

	// Write data to the connection
	_, err = conn.Write([]byte("Hello, world!")) // replace with your data
	if err != nil {
		panic(err)
	}
}

