package dtls

import (
	"context"
	"fmt"
	"net"

	dd "github.com/refraction-networking/conjure/application/lib"
	pb "github.com/refraction-networking/gotapdance/protobuf"
)

type Transport struct{}

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

// Connect takes a registraion and returns a dtls Conn connected to the client
func (t *Transport) Connect(ctx context.Context, reg *dd.DecoyRegistration) (net.Conn, error) {
	if reg.Transport != pb.TransportType_Dtls {
		return nil, fmt.Errorf("not dtls transport: %v", reg.Transport)
	}
	return nil, fmt.Errorf("not implemented")
}
