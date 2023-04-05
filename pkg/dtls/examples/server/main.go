package main

import (
	"flag"
	"fmt"
	"net"

	"github.com/mingyech/dtls/v2/examples/util"
	"github.com/refraction-networking/conjure/pkg/dtls"
)

func main() {
	var port = flag.Int("port", 6666, "port to listen on")
	var secret = flag.String("secret", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", "shared secret")
	flag.Parse()

	// Prepare the IP to connect to
	addr := &net.UDPAddr{Port: *port}

	listener, err := dtls.Listen(addr)
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
