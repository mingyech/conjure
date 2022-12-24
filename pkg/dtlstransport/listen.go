package dtlstransport

import (
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"sync"

	"github.com/pion/dtls/v2"
	"github.com/pion/logging"
	"github.com/pion/sctp"
)

// Listener represents a DTLS Listener
type Listener struct {
	dtlsListener    net.Listener
	connMap         map[string](chan net.Conn)
	connMapMutex    sync.RWMutex
	connToCert      map[string]*certPair
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
		connMap:     map[string](chan net.Conn){},
		connToCert:  map[string]*certPair{},
		defaultCert: defaultCert,
	}

	// Prepare the configuration of the DTLS connection
	config := &dtls.Config{
		ExtendedMasterSecret:  dtls.RequireExtendedMasterSecret,
		ClientAuth:            dtls.RequireAnyClientCert,
		VerifyPeerCertificate: newDTLSListner.verifyCertificate,
		GetCertificate:        newDTLSListner.getCertificateFromClientHello,
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
	if clientHello.ServerName == "" {
		return l.defaultCert, nil
	}

	l.connToCertMutex.RLock()
	defer l.connToCertMutex.RUnlock()

	return l.connToCert[clientHello.ServerName].serverCert, nil
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

			rawCerts := newDTLSConn.ConnectionState().PeerCertificates
			connID, err := connIDFromCertificate(rawCerts[0])
			if err != nil {
				return
			}

			// Wrap SCTP on top of DTLS connection
			sctpConfig := sctp.Config{
				NetConn:       newDTLSConn,
				LoggerFactory: logging.NewDefaultLoggerFactory(),
			}

			sctpServer, err := sctp.Server(sctpConfig)
			if err != nil {
				return
			}

			sctpStream, err := sctpServer.AcceptStream()
			if err != nil {
				return
			}

			sctpConn := &sctpConn{Stream: sctpStream, DTLSConn: newDTLSConn}

			l.connMapMutex.Lock()
			defer l.connMapMutex.Unlock()

			acceptChan, ok := l.connMap[connID]

			if !ok {
				return
			}

			acceptChan <- sctpConn

			close(acceptChan)
			delete(l.connMap, connID)
		}()
	}
}

func (l *Listener) verifyCertificate(rawCerts [][]byte, verifiedChains [][]*x509.Certificate) error {
	if verifiedChains != nil {
		return fmt.Errorf("unexpected non-nil verified chain")
	}

	if len(rawCerts) != 1 {
		return fmt.Errorf("expected 1 self-signed certificate, got %v", len(rawCerts))
	}

	// Check if incomming client certificate was registered
	connID, err := connIDFromCertificate(rawCerts[0])
	if err != nil {
		return fmt.Errorf("error getting conn ID from certificate: %v", err)
	}

	l.connMapMutex.RLock()
	defer l.connMapMutex.RUnlock()

	if l.connMap[connID] == nil {
		return fmt.Errorf("no registered connection ID with certificate")
	}

	// All good
	return nil
}

// AcceptFromSeed accepts a connection with a seed
func (l *Listener) AcceptFromSeed(secret []byte) (net.Conn, error) {
	clientCert, serverCert, err := certsFromSeed(secret)
	if err != nil {
		return &dtls.Conn{}, fmt.Errorf("error generating certificatess from seed: %v", err)
	}

	connID, err := connIDFromCertificate(clientCert.Certificate[0])

	l.connToCertMutex.Lock()

	l.connToCert[connID] = &certPair{clientCert: clientCert, serverCert: serverCert}
	l.connToCertMutex.Unlock()

	if err != nil {
		return &dtls.Conn{}, err
	}

	l.connMapMutex.Lock()

	if l.connMap[connID] != nil {
		return &dtls.Conn{}, fmt.Errorf("seed has already been registered")
	}

	connChan := make(chan net.Conn)
	l.connMap[connID] = connChan
	l.connMapMutex.Unlock()

	conn := <-connChan

	return conn, nil

}

func connIDFromCertificate(cert []byte) (string, error) {
	certDer, err := x509.ParseCertificate(cert)

	if err != nil {
		return "", fmt.Errorf("error parsing generated client certificate: %v", err)
	}
	return string(certDer.Subject.CommonName), nil
}
