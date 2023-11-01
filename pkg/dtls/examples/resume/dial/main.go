package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"flag"
	"fmt"
	"net"

	pion_dtls "github.com/pion/dtls/v2"
	"github.com/pion/dtls/v2/examples/util"
	"github.com/pion/dtls/v2/pkg/protocol/handshake"
	"github.com/refraction-networking/conjure/pkg/dtls"
)

func main() {
	var remoteAddr = flag.String("saddr", "127.0.0.1:6666", "remote address")
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

	clientCert, serverCert, err := dtls.CertsFromSeed(sharedSecret)

	if err != nil {
		panic(err)
	}

	clientHelloRandom, err := dtls.ClientHelloRandomFromSeed(sharedSecret)
	if err != nil {
		panic(err)
	}

	verifyServerCertificate := func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(rawCerts) != 1 {
			return fmt.Errorf("expected 1 peer certificate, got %v", len(rawCerts))
		}

		err := dtls.VerifyCert(rawCerts[0], serverCert.Certificate[0])
		if err != nil {
			return fmt.Errorf("error verifying server certificate: %v", err)
		}

		return nil
	}

	// Prepare the configuration of the DTLS connection
	dtlsConf := &pion_dtls.Config{
		Certificates:            []tls.Certificate{*clientCert},
		ExtendedMasterSecret:    pion_dtls.RequireExtendedMasterSecret,
		CustomClientHelloRandom: func() [handshake.RandomBytesLength]byte { return clientHelloRandom },

		// We use VerifyPeerCertificate to authenticate the peer's certificate. This is necessary as Go's non-deterministic ECDSA signatures and hash comparison method for self-signed certificates can cause verification failure.
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: verifyServerCertificate,
	}

	dtlsConn, err := pion_dtls.ClientWithContext(context.Background(), udpConn, dtlsConf)
	if err != nil {
		panic(err)
	}

	fmt.Println("Connected; type 'exit' to shutdown gracefully")

	// Simulate a chat session
	util.Chat(dtlsConn)

}
