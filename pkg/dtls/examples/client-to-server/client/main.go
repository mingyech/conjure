package main

import (
	"context"
	"flag"
	"fmt"
	"net"

	"github.com/mingyech/dtls/v2/examples/util"
	"github.com/refraction-networking/conjure/pkg/dtls"
)

func main() {
	var remoteAddr = flag.String("saddr", "", "remote address")
	var localAddr = flag.String("laddr", "", "source address")
	var secret = flag.String("secret", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", "shared secret")
	flag.Parse()
	// Prepare the IP to connect to
	laddr, err := net.ResolveUDPAddr("udp", *localAddr)
	util.Check(err)

	addr, err := net.ResolveUDPAddr("udp", *remoteAddr)
	util.Check(err)

	sharedSecret := []byte(*secret)

	udpConn, err := net.DialUDP("udp", laddr, addr)
	util.Check(err)

	dtlsConn, err := dtls.ClientWithContext(context.Background(), udpConn, sharedSecret)
	util.Check(err)

	fmt.Println("Connected; type 'exit' to shutdown gracefully")

	// Simulate a chat session
	util.Chat(dtlsConn)

}
