package dtls

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"

	"github.com/mingyech/dtls/v2"
	"github.com/mingyech/dtls/v2/pkg/protocol/handshake"
	"github.com/pion/logging"
	"github.com/pion/sctp"
)

// Dial creates a DTLS connection to the given network address using the given shared secret
func Dial(remoteAddr *net.UDPAddr, secret []byte) (net.Conn, error) {
	return DialContext(context.Background(), remoteAddr, secret)
}

// DialContext creates a DTLS connection to the given network address using the given shared secret
func DialContext(ctx context.Context, remoteAddr *net.UDPAddr, seed []byte) (net.Conn, error) {
	clientCert, serverCert, err := certsFromSeed(seed)

	if err != nil {
		return nil, fmt.Errorf("error generating certs: %v", err)
	}

	certPool := x509.NewCertPool()

	serverCertDer, err := x509.ParseCertificate(serverCert.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("error parsing cert: %v", err)
	}

	certPool.AddCert(serverCertDer)

	clientHelloRandom, err := clientHelloRandomFromSeed(seed)
	if err != nil {
		return nil, fmt.Errorf("error generating client hello random: %v", err)
	}
	fmt.Println(string(clientHelloRandom[:]))

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
		RootCAs:                 certPool,
		CustomClientHelloRandom: func() [handshake.RandomBytesLength]byte { return clientHelloRandom },

		// we verify the peer's cert using VerifyPeerCertificate, because go does not generate dertiministic
		// ecdsa signatures in certificate and checks self signed certificate by comparing their hashes,
		// so the verification fails unless we check the signature without using hashes ourselves
		InsecureSkipVerify:    true,
		VerifyPeerCertificate: verifyServerCertificate,
	}

	// Connect to a DTLS server
	dtlsConn, err := dtls.DialWithContext(ctx, "udp", remoteAddr, config)
	if err != nil {
		return nil, fmt.Errorf("error creating dtls connection: %v", err)
	}

	// Start SCTP
	sctpConf := sctp.Config{
		NetConn:       dtlsConn,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	}

	sctpClient, err := sctp.Client(sctpConf)

	if err != nil {
		return nil, fmt.Errorf("error creating sctp client: %v", err)
	}

	sctpRWC, err := sctpClient.OpenStream(0, sctp.PayloadTypeWebRTCString)

	if err != nil {
		return nil, fmt.Errorf("error setting up stream: %v", err)
	}

	sctpConn := &sctpConn{Stream: sctpRWC, DTLSConn: dtlsConn}

	return sctpConn, nil
}
