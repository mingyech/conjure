package dtls

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"sync"

	"github.com/mingyech/dtls/v2"
	"github.com/mingyech/dtls/v2/pkg/protocol/handshake"
	"github.com/pion/logging"
	"github.com/pion/sctp"
)

// Listener represents a DTLS Listener
type Listener struct {
	dtlsListener    net.Listener
	connMap         map[[handshake.RandomBytesLength]byte](chan net.Conn)
	connMapMutex    sync.RWMutex
	connToCert      map[[handshake.RandomBytesLength]byte]*certPair
	connToCertMutex sync.RWMutex
	defaultCert     *tls.Certificate
}

type certPair struct {
	clientCert *tls.Certificate
	serverCert *tls.Certificate
}

// Listen creates a listener and starts listening
func Listen(addr *net.UDPAddr) (*Listener, error) {

	// the default cert is only used for checking avaliable cipher suites
	defaultCert, err := randomCertificate()
	if err != nil {
		return nil, fmt.Errorf("error generating default random cert: %v", err)
	}

	newDTLSListner := Listener{
		connMap:     map[[handshake.RandomBytesLength]byte](chan net.Conn){},
		connToCert:  map[[handshake.RandomBytesLength]byte]*certPair{},
		defaultCert: defaultCert,
	}

	// Prepare the configuration of the DTLS connection
	config := &dtls.Config{
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
		ClientAuth:           dtls.RequireAnyClientCert,
		GetCertificate:       newDTLSListner.getCertificateFromClientHello,
		VerifyConnection:     newDTLSListner.verifyConnection,
	}

	listener, err := dtls.Listen("udp", addr, config)
	if err != nil {
		return &Listener{}, fmt.Errorf("error listening to dtls: %v", err)
	}

	newDTLSListner.dtlsListener = listener

	go newDTLSListner.acceptLoop()

	return &newDTLSListner, nil
}

func randomCertificate() (*tls.Certificate, error) {
	seedBytes := []byte{}
	_, err := rand.Read(seedBytes)
	if err != nil {
		return nil, err
	}
	return newCertificate(seedBytes)
}

func (l *Listener) getCertificateFromClientHello(clientHello *dtls.ClientHelloInfo) (*tls.Certificate, error) {
	// This function is sometimes called by the dtls library to get the availiable ciphersuites,
	// respond with a default certificate with the availible ciphersuites
	if clientHello.CipherSuites == nil {
		return l.defaultCert, nil
	}

	l.connToCertMutex.RLock()
	defer l.connToCertMutex.RUnlock()

	certs, ok := l.connToCert[clientHello.RandomBytes]

	if !ok {
		// Respond with random server certificate if not registered, will reject client cert later during handshake
		randomCert, err := randomCertificate()
		if err != nil {
			return nil, fmt.Errorf("failed to generate random certificate: %v", err)
		}

		return randomCert, nil
	}

	return certs.serverCert, nil
}

func wrapSCTP(conn net.Conn) (net.Conn, error) {

	// Start SCTP over DTLS connection
	sctpConfig := sctp.Config{
		NetConn:       conn,
		LoggerFactory: logging.NewDefaultLoggerFactory(),
	}

	sctpServer, err := sctp.Server(sctpConfig)
	if err != nil {
		return nil, err
	}

	sctpStream, err := sctpServer.AcceptStream()
	if err != nil {
		return nil, err
	}
	return &sctpConn{Stream: sctpStream, DTLSConn: conn}, nil

}

func ServerWithContext(ctx context.Context, conn net.Conn, sharedSecret []byte) (net.Conn, error) {

	clientCert, serverCert, err := certsFromSeed(sharedSecret)
	if err != nil {
		return nil, fmt.Errorf("error generating certificatess from seed: %v", err)
	}

	VerifyPeerCertificate := func(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {

		err := verifyCert(rawCerts[0], clientCert.Certificate[0])
		if err != nil {
			return fmt.Errorf("error verifying peer certificate: %v", err)
		}

		return nil
	}

	config := &dtls.Config{
		ExtendedMasterSecret:  dtls.RequireExtendedMasterSecret,
		ClientAuth:            dtls.RequireAnyClientCert,
		Certificates:          []tls.Certificate{*serverCert},
		VerifyPeerCertificate: VerifyPeerCertificate,
	}

	dtlsConn, err := dtls.ServerWithContext(ctx, conn, config)

	if err != nil {
		return nil, err
	}

	sctpConn, err := wrapSCTP(dtlsConn)

	if err != nil {
		return nil, err
	}

	return sctpConn, nil

}

func (l *Listener) acceptLoop() {
	for {
		newConn, err := l.dtlsListener.Accept()
		if err != nil {
			continue
		}

		go func() {
			newDTLSConn, ok := newConn.(*dtls.Conn)
			if !ok {
				return
			}

			connState := newDTLSConn.ConnectionState()
			connID := connState.RemoteRandomBytes()

			sctpConn, err := wrapSCTP(newDTLSConn)
			if err != nil {
				return
			}

			l.connMapMutex.RLock()
			defer l.connMapMutex.RUnlock()

			acceptChan, ok := l.connMap[connID]

			if !ok {
				return
			}

			acceptChan <- sctpConn

			close(acceptChan)
		}()
	}
}

func (l *Listener) verifyConnection(state *dtls.State) error {

	certs, ok := l.connToCert[state.RemoteRandomBytes()]
	if !ok {
		return fmt.Errorf("no matching certificate found with client hello random")
	}

	if len(state.PeerCertificates) != 1 {
		return fmt.Errorf("expected 1 peer certificate, got %v", len(state.PeerCertificates))
	}

	err := verifyCert(state.PeerCertificates[0], certs.clientCert.Certificate[0])
	if err != nil {
		return fmt.Errorf("error verifying peer certificate: %v", err)
	}

	return nil
}

func verifyCert(cert, correct []byte) error {
	incommingCert, err := x509.ParseCertificate(cert)
	if err != nil {
		return fmt.Errorf("error parsing peer certificate: %v", err)
	}

	correctCert, err := x509.ParseCertificate(correct)
	if err != nil {
		return fmt.Errorf("error parsing correct certificate: %v", err)
	}

	correctCert.KeyUsage = x509.KeyUsageCertSign // CheckSignature have requirements for the KeyUsage field
	err = incommingCert.CheckSignatureFrom(correctCert)
	if err != nil {
		return fmt.Errorf("error verifying certificate signature: %v", err)
	}

	return nil
}

// AcceptFromSecret accepts a connection with shared secret
func (l *Listener) AcceptFromSecret(secret []byte) (net.Conn, error) {
	clientCert, serverCert, err := certsFromSeed(secret)
	if err != nil {
		return &dtls.Conn{}, fmt.Errorf("error generating certificatess from seed: %v", err)
	}

	connID, err := clientHelloRandomFromSeed(secret)
	if err != nil {
		return &dtls.Conn{}, err
	}

	l.registerCert(connID, clientCert, serverCert)
	defer l.removeCert(connID)

	connChan, err := l.registerChannel(connID)
	if err != nil {
		return nil, fmt.Errorf("error registering channel: %v", err)
	}
	defer l.removeChannel(connID)

	conn := <-connChan

	return conn, nil
}

func (l *Listener) registerCert(connID [handshake.RandomBytesLength]byte, clientCert, serverCert *tls.Certificate) {
	l.connToCertMutex.Lock()
	defer l.connToCertMutex.Unlock()
	l.connToCert[connID] = &certPair{clientCert: clientCert, serverCert: serverCert}
}

func (l *Listener) removeCert(connID [handshake.RandomBytesLength]byte) {
	l.connToCertMutex.Lock()
	defer l.connToCertMutex.Unlock()
	delete(l.connToCert, connID)
}

func (l *Listener) registerChannel(connID [handshake.RandomBytesLength]byte) (<-chan net.Conn, error) {
	l.connMapMutex.Lock()
	defer l.connMapMutex.Unlock()

	if l.connMap[connID] != nil {
		return nil, fmt.Errorf("seed already registered")
	}

	connChan := make(chan net.Conn)
	l.connMap[connID] = connChan

	return connChan, nil
}

func (l *Listener) removeChannel(connID [handshake.RandomBytesLength]byte) {
	l.connMapMutex.Lock()
	defer l.connMapMutex.Unlock()

	delete(l.connMap, connID)
}
