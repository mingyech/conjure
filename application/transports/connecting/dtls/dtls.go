package dtls

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/libp2p/go-reuseport"
	dd "github.com/refraction-networking/conjure/application/lib"
	"github.com/refraction-networking/conjure/application/log"
	"github.com/refraction-networking/conjure/application/transports"
	"github.com/refraction-networking/conjure/pkg/dtls"
	pb "github.com/refraction-networking/gotapdance/protobuf"
	"google.golang.org/protobuf/types/known/anypb"
)

const (
	// port range boundaries for min when randomizing
	portRangeMin = 1024
	portRangeMax = 65535
	defaultPort  = 443
)

const listenPort = 41245

type Transport struct {
	dnat         *transports.DNAT
	dtlsListener *dtls.Listener
}

// Name returns name of the transport
func (Transport) Name() string {
	return "dtls"
}

// LogPrefix returns log prefix of the transport
func (Transport) LogPrefix() string {
	return "DTLS"
}

// GetIdentifier returns an identifier unique a registration
func (Transport) GetIdentifier(reg *dd.DecoyRegistration) string {
	return string(reg.Keys.ConjureHMAC("dtlsTrasportHMACString"))
}

// NewTransport creates a new dtls transport
func NewTransport() (*Transport, error) {
	addr := &net.UDPAddr{Port: listenPort}

	listener, err := dtls.Listen(addr)
	if err != nil {
		return nil, fmt.Errorf("error creating dtls listner: %v", err)
	}

	dnat, err := transports.NewDNAT()

	if err != nil {
		return nil, fmt.Errorf("error connecting to tun device for DNAT: %v", err)
	}

	return &Transport{
		dnat:         dnat,
		dtlsListener: listener,
	}, nil
}

// Connect takes a registraion and returns a dtls Conn connected to the client
func (t *Transport) Connect(ctx context.Context, reg *dd.DecoyRegistration) (net.Conn, error) {
	if reg.Transport != pb.TransportType_DTLS {
		return nil, transports.ErrNotTransport
	}

	clientAddr := net.UDPAddr{IP: net.ParseIP(reg.GetRegistrationAddress()), Port: int(reg.GetSrcPort())}

	t.dnat.AddEntry(clientAddr.IP, uint16(clientAddr.Port), reg.PhantomIp, reg.PhantomPort)

	laddr := net.UDPAddr{IP: net.ParseIP("0.0.0.0"), Port: listenPort}

	udpConn, err := reuseport.Dial("udp", laddr.String(), clientAddr.String())
	if err != nil {
		return nil, fmt.Errorf("error dialing client: %v", err)
	}

	ctxtimeout, cancel := context.WithTimeout(context.Background(), 7*time.Second)
	defer cancel()

	dtlsConn, err := dtls.ClientWithContext(ctxtimeout, udpConn, reg.Keys.SharedSecret)
	if err != nil {
		log.Debugf("error connecting to dtls client: %v, fallback to listen\n", err)
		conn, err := t.dtlsListener.AcceptFromSecret(reg.Keys.SharedSecret)
		if err != nil {
			return nil, fmt.Errorf("error accepting dtls connection from secret: %v", err)
		}
		return conn, nil
	}

	return dtlsConn, nil
}

func (Transport) GetSrcPort(libVersion uint, seed []byte, params any) (uint16, error) {
	parameters, ok := params.(*pb.DTLSTransportParams)
	if !ok {
		return 0, fmt.Errorf("bad parameters provided")
	}

	return uint16(parameters.GetSrcPort()), nil
}

func (Transport) GetDstPort(libVersion uint, seed []byte, params any) (uint16, error) {
	return transports.PortSelectorRange(portRangeMin, portRangeMax, seed)
}

func (Transport) GetProto() pb.IPProto {
	return pb.IPProto_Udp
}

func (Transport) ParseParams(libVersion uint, data *anypb.Any) (any, error) {
	var m = &pb.DTLSTransportParams{}
	err := transports.UnmarshalAnypbTo(data, m)
	return m, err
}
