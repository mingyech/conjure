package dtls

import (
	"net"
	"time"

	"github.com/pion/dtls/v2"
	"github.com/pion/sctp"
)

// sctpConn implements the net.Conn interface using sctp stream and DTLS conn
type sctpConn struct {
	*sctp.Stream
	DTLSConn *dtls.Conn
}

func (s *sctpConn) LocalAddr() net.Addr {
	return s.DTLSConn.LocalAddr()
}

func (s *sctpConn) RemoteAddr() net.Addr {
	return s.DTLSConn.RemoteAddr()
}

func (s *sctpConn) SetDeadline(t time.Time) error {
	return s.DTLSConn.SetDeadline(t)
}

func (s *sctpConn) SetWriteDeadline(t time.Time) error {
	return s.DTLSConn.SetWriteDeadline(t)
}
