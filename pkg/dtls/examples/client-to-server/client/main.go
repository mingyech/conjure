package main

import (
	"flag"
	"fmt"
	"net"
	"os"

	"github.com/mingyech/dtls/v2/examples/util"
	"github.com/refraction-networking/conjure/pkg/dtls"
)

func main() {
	var ip = flag.String("ip", "127.0.0.1", "ip to connect to")
	var port = flag.Int("port", 6666, "port to connect to")
	var secret = flag.String("secret", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", "shared secret")
	flag.Parse()
	// Prepare the IP to connect to
	addr := &net.UDPAddr{IP: net.ParseIP(*ip), Port: *port}

	sharedSecret := []byte(*secret)

	dtlsConn, err := dtls.Dial(addr, sharedSecret)

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Connected; type 'exit' to shutdown gracefully")

	// Simulate a chat session
	util.Chat(dtlsConn)

}
