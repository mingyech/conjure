package dtls

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/libp2p/go-reuseport"
	dd "github.com/refraction-networking/conjure/application/lib"
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

	connCh := make(chan net.Conn, 2)
	errCh := make(chan error, 2)

	go func() {
		udpConn, err := reuseport.Dial("udp", laddr.String(), clientAddr.String())
		if err != nil {
			errCh <- fmt.Errorf("error connecting to dtls client: %v", err)
			return
		}

		dtlsConn, err := dtls.ClientWithContext(ctx, udpConn, reg.Keys.SharedSecret)
		if err != nil {
			errCh <- fmt.Errorf("error connecting to dtls client: %v", err)
			return
		}

		connCh <- dtlsConn
	}()

	go func() {
		conn, err := t.dtlsListener.AcceptFromSecretWithContext(ctx, reg.Keys.SharedSecret)
		if err != nil {
			errCh <- fmt.Errorf("error accepting dtls connection from secret: %v", err)
			return
		}

		connCh <- conn
	}()

	var errs []error
	for i := 0; i < 2; i++ {
		select {
		case conn := <-connCh:
			if conn != nil {
				return conn, nil // success, so return the connection
			}
		case err := <-errCh:
			if err != nil { // store the error
				errs = append(errs, err)
			}
		}
	}

	// combine errors into a single error
	var combinedErr error
	if len(errs) > 0 {
		errStrings := make([]string, len(errs))
		for i, err := range errs {
			errStrings[i] = err.Error()
		}
		combinedErr = fmt.Errorf(strings.Join(errStrings, "; "))
	}

	return nil, combinedErr // if we reached here, both attempts failed
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
