package dtls

import (
	"context"
	"fmt"
	"net"

	dd "github.com/refraction-networking/conjure/application/lib"
	"github.com/refraction-networking/conjure/application/transports"
	"github.com/refraction-networking/conjure/pkg/dtls"
	pb "github.com/refraction-networking/gotapdance/protobuf"
	"google.golang.org/protobuf/types/known/anypb"
)

const port int = 41245

type Transport struct {
	dtlsListener *dtls.Listener
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

// NewTransport creates a new dtls transport
func NewTransport() (*Transport, error) {
	addr := &net.UDPAddr{Port: port}

	listener, err := dtls.Listen(addr)
	if err != nil {
		return nil, fmt.Errorf("error creating dtls listner: %v", err)
	}

	return &Transport{
		dtlsListener: listener,
	}, nil
}

// Connect takes a registraion and returns a dtls Conn connected to the client
func (t *Transport) Connect(ctx context.Context, reg *dd.DecoyRegistration) (net.Conn, error) {
	if reg.Transport != pb.TransportType_DTLS {
		return nil, transports.ErrNotTransport
	}

	conn, err := t.dtlsListener.AcceptFromSecret(reg.Keys.SharedSecret)
	if err != nil {
		return nil, fmt.Errorf("error accepting dtls connection from secret: %v", err)
	}

	return conn, nil
}

func (t Transport) GetDstPort(libVersion uint, seed []byte, params any) (uint16, error) {
	return uint16(port), nil
}

func (Transport) GetProto() pb.IPProto {
	return pb.IPProto_Udp
}

func (Transport) ParseParams(libVersion uint, data *anypb.Any) (any, error) {
	return nil, nil
}
