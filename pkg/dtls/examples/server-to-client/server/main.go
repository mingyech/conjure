package main

import (
	"flag"
	"fmt"
	"net"

	"github.com/mingyech/dtls/v2/examples/util"
	"github.com/refraction-networking/conjure/pkg/dtls"
)

const defaultSTUNServer = "stun.voip.blackberry.com:3478"

func main() {
	// var localAddr = flag.String("laddr", "", "source address")
	var secret = flag.String("secret", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", "shared secret")
	var stunServer = flag.String("stun-server", defaultSTUNServer, "STUN server for NAT traversal.")
	flag.Parse()

	privPort, pubPort, err := PublicAddr(*stunServer)
	util.Check(err)
	fmt.Printf("Public Port: %v, Private port: %v\n", pubPort, privPort)

	listener, err := dtls.Listen(&net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: privPort})
	if err != nil {
		fmt.Printf("error creating dtls listner: %v\n", err)
	}

	fmt.Println("Listening")

	// Simulate a chat session
	hub := util.NewHub()

	sharedSecret := []byte(*secret)
	go func() {
		for {
			// Wait for a connection.
			conn, err := listener.AcceptFromSecret(sharedSecret)
			util.Check(err)

			fmt.Println("new connection")
			// defer conn.Close() // TODO: graceful shutdown

			// `conn` is of type `net.Conn` but may be casted to `dtls.Conn`
			// using `dtlsConn := conn.(*dtls.Conn)` in order to to expose
			// functions like `ConnectionState` etc.

			// Register the connection with the chat hub
			hub.Register(conn)
		}
	}()

	// Start chatting
	hub.Chat()
}
