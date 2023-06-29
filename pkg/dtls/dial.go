package dtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"

	"github.com/mingyech/dtls/v2"
	"github.com/mingyech/dtls/v2/pkg/protocol/handshake"
)

// Dial creates a DTLS connection to the given network address using the given shared secret
func Dial(remoteAddr *net.UDPAddr, secret []byte) (net.Conn, error) {
	return DialWithContext(context.Background(), remoteAddr, secret)
}

func DialWithContext(ctx context.Context, remoteAddr *net.UDPAddr, seed []byte) (net.Conn, error) {
	conn, err := net.DialUDP("udp", nil, remoteAddr)
	if err != nil {
		return nil, err
	}

	return ClientWithContext(ctx, conn, seed)
}

// DialWithContext creates a DTLS connection to the given network address using the given shared secret
func ClientWithContext(ctx context.Context, conn net.Conn, seed []byte) (net.Conn, error) {
	clientCert, serverCert, err := certsFromSeed(seed)

	if err != nil {
		return nil, fmt.Errorf("error generating certs: %v", err)
	}

	clientHelloRandom, err := clientHelloRandomFromSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("error generating client hello random: %v", err)
	}

	verifyServerCertificate := func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
		if len(rawCerts) != 1 {
			return fmt.Errorf("expected 1 peer certificate, got %v", len(rawCerts))
		}

		err := verifyCert(rawCerts[0], serverCert.Certificate[0])
		if err != nil {
			return fmt.Errorf("error verifying server certificate: %v", err)
		}

		return nil
	}

	// Prepare the configuration of the DTLS connection
	config := &dtls.Config{
		Certificates:            []tls.Certificate{*clientCert},
		ExtendedMasterSecret:    dtls.RequireExtendedMasterSecret,
		CustomClientHelloRandom: func() [handshake.RandomBytesLength]byte { return clientHelloRandom },

		// We use VerifyPeerCertificate to authenticate the peer's certificate. This is necessary as Go's non-deterministic ECDSA signatures and hash comparison method for self-signed certificates can cause verification failure.
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: verifyServerCertificate,
	}

	dtlsConn, err := dtls.ClientWithContext(ctx, conn, config)

	if err != nil {
		return nil, fmt.Errorf("error creating dtls connection: %v", err)
	}

	return dtlsConn, nil
}
