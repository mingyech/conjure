package dtlstransport

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"time"

	"github.com/pion/dtls/v2"
	"github.com/pion/logging"
	"github.com/pion/sctp"
)

// Dial creates a DTLS connection to the given network address using the given shared secret
func Dial(remoteAddr *net.UDPAddr, secret []byte) (net.Conn, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return DialWithContext(ctx, remoteAddr, secret)
}

// DialWithContext creates a DTLS connection to the given network address using the given shared secret
func DialWithContext(ctx context.Context, remoteAddr *net.UDPAddr, seed []byte) (net.Conn, error) {
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

	// Prepare the configuration of the DTLS connection
	config := &dtls.Config{
		Certificates:         []tls.Certificate{*clientCert},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
		RootCAs:              certPool,
		ServerName:           serverCertDer.Subject.CommonName,
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
