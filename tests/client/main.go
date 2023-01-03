package main

import (
	"fmt"
	"net"
	"os"

	"github.com/mingyech/dtls/v2/examples/util"
	"github.com/refraction-networking/conjure/pkg/dtls"
)

func main() {
	// Prepare the IP to connect to
	addr := &net.UDPAddr{IP: net.ParseIP("127.0.0.1"), Port: 4444}

	sharedSecret := []byte(`1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef`)

	dtlsConn, err := dtls.Dial(addr, sharedSecret)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Connected; type 'exit' to shutdown gracefully")

	// Simulate a chat session
	util.Chat(dtlsConn)

}
