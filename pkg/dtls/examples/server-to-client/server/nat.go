package main

import (
	"fmt"
	"net"
	"syscall"

	"github.com/pion/stun"
)

const ttl = 3

func openUDP(laddr, addr *net.UDPAddr) error {
	// Create a UDP connection
	conn, err := net.DialUDP("udp", laddr, addr)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Get the file descriptor
	fd, err := conn.File()
	if err != nil {
		return err
	}
	defer fd.Close()

	// Set the TTL
	err = syscall.SetsockoptInt(int(fd.Fd()), syscall.IPPROTO_IP, syscall.IP_TTL, ttl)
	if err != nil {
		return err
	}

	// Write data to the connection
	_, err = conn.Write([]byte(""))
	if err != nil {
		return err
	}

	conn.Close()

	// No error
	return nil
}

var (
	privPortSingle int
	pubPortSingle  int
)

func PublicAddr(stunServer string) (privatePort int, publicPort int, err error) {

	if privPortSingle != 0 && pubPortSingle != 0 {
		return privPortSingle, pubPortSingle, nil
	}

	udpConn, err := net.Dial("udp", stunServer)
	if err != nil {
		return 0, 0, fmt.Errorf("error connecting to STUN server: %v", err)
	}
	defer udpConn.Close()

	localAddr, err := net.ResolveUDPAddr(udpConn.LocalAddr().Network(), udpConn.LocalAddr().String())
	if err != nil {
		return 0, 0, fmt.Errorf("error resolving local address: %v", err)
	}

	client, err := stun.NewClient(udpConn)
	if err != nil {
		return 0, 0, fmt.Errorf("error creating STUN client: %v", err)
	}

	message := stun.MustBuild(stun.TransactionID, stun.BindingRequest)

	var xorAddr stun.XORMappedAddress

	err = client.Do(message, func(res stun.Event) {
		if res.Error != nil {
			err = res.Error
			return
		}

		err = xorAddr.GetFrom(res.Message)
		if err != nil {
			return
		}
	})

	if err != nil {
		return 0, 0, fmt.Errorf("error getting address from STUN: %v", err)
	}

	privPortSingle = localAddr.Port
	pubPortSingle = xorAddr.Port

	return localAddr.Port, xorAddr.Port, nil
}
