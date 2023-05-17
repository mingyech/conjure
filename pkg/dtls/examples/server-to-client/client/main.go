package main

import (
	"context"
	"flag"
	"fmt"
	"net"

	"github.com/libp2p/go-reuseport"
	"github.com/mingyech/dtls/v2/examples/util"
	"github.com/refraction-networking/conjure/application/transports"
	"github.com/refraction-networking/conjure/pkg/dtls"
)

func main() {
	var remoteAddr = flag.String("raddr", "", "remote address")
	var localAddr = flag.String("laddr", "", "source address")
	var phantomAddr = flag.String("paddr", "", "phantom address")
	var secret = flag.String("secret", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", "shared secret")
	flag.Parse()
	// Prepare the IP to connect to
	// laddr, err := net.ResolveUDPAddr("udp", *localAddr)
	// util.Check(err)

	addr, err := net.ResolveUDPAddr("udp", *remoteAddr)
	util.Check(err)

	paddr, err := net.ResolveUDPAddr("udp", *phantomAddr)
	util.Check(err)

	dnat, err := transports.NewDNAT("tun0")
	util.Check(err)

	dnat.AddEntry(addr.IP, uint16(addr.Port), paddr.IP, uint16(paddr.Port))

	sharedSecret := []byte(*secret)

	udpConn, err := reuseport.Dial("udp", *localAddr, *remoteAddr)
	util.Check(err)

	dtlsConn, err := dtls.ClientWithContext(context.Background(), udpConn, sharedSecret)
	util.Check(err)

	fmt.Println("Connected; type 'exit' to shutdown gracefully")

	// Simulate a chat session
	util.Chat(dtlsConn)

}
