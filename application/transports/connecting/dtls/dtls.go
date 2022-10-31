package dtls

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/pion/dtls/v2"
	"github.com/pion/dtls/v2/pkg/crypto/selfsign"
	dd "github.com/refraction-networking/conjure/application/lib"
	"github.com/refraction-networking/conjure/application/transports"
	pb "github.com/refraction-networking/gotapdance/protobuf"
)

const testPort int = 4443

type Transport struct {
	dtlsListener net.Listener
	connMap      map[string]chan net.Conn
	mapMutex     sync.RWMutex
}

// Name returns name of the transport
func (t Transport) Name() string {
	return "dtls"
}

// LogPrefix returns log prefix of the transport
func (t Transport) LogPrefix() string {
	return "DTLS"
}

// GetIdentifier returns an identifier unique a registration
func (t Transport) GetIdentifier(reg *dd.DecoyRegistration) string {
	return string(reg.Keys.ConjureHMAC("dtlsTrasportHMACString"))
}

// NewDTLSTransport creates a new dtls transport
func NewDTLSTransport() (*Transport, error) {
	udpAddr := &net.UDPAddr{IP: nil, Port: testPort}

	certificate, err := selfsign.GenerateSelfSigned()

	if err != nil {
		return nil, fmt.Errorf("failed to create udp conn: %w", err)
	}
	// Create parent context to cleanup handshaking connections on exit.
	ctx, _ := context.WithCancel(context.Background())

	// Prepare the configuration of the DTLS connection
	config := &dtls.Config{
		Certificates:         []tls.Certificate{certificate},
		ExtendedMasterSecret: dtls.RequireExtendedMasterSecret,
		// Create timeout context for accepted connection.
		ConnectContextMaker: func() (context.Context, func()) {
			return context.WithTimeout(ctx, 30*time.Second)
		},
	}

	listener, err := dtls.Listen("udp", udpAddr, config)

	if err != nil {
		return nil, fmt.Errorf("failed to create udp conn: %w", err)
	}

	return &Transport{
		dtlsListener: listener,
		connMap:      make(map[string]chan net.Conn),
		mapMutex:     sync.RWMutex{},
	}, nil
}

func (t *Transport) acceptLoop() {
	for {
		newConn, err := t.dtlsListener.Accept()
		if err != nil {
			continue
		}

		go func() {
			t.mapMutex.lock()
			if t.connMap[newConn.RemoteAddr().String()] == nil {
				t.connMap[newConn.RemoteAddr().String()] = make(chan net.Conn)
			}
		}()
	}
}

// Connect takes a registraion and returns a dtls Conn connected to the client
func (t *Transport) Connect(ctx context.Context, reg *dd.DecoyRegistration) (net.Conn, error) {
	if reg.Transport != pb.TransportType_Dtls {
		return nil, transports.ErrNotTransport
	}

	return nil, fmt.Errorf("not implemented")
}
