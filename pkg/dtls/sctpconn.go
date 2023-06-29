package dtls

import (
	"io"
	"net"
	"time"
)

// sctpConn implements the net.Conn interface using sctp stream and DTLS conn
type sctpConn struct {
	stream io.ReadWriteCloser
	conn   net.Conn
}

func newSCTPConn(stream io.ReadWriteCloser, conn net.Conn) *sctpConn {
	return &sctpConn{stream: stream, conn: conn}
}

func (s *sctpConn) Close() error {
	err := s.stream.Close()
	if err != nil {
		return err
	}
	return s.conn.Close()
}

func (s *sctpConn) Write(b []byte) (int, error) {
	return s.stream.Write(b)
}

func (s *sctpConn) Read(b []byte) (int, error) {
	return s.stream.Read(b)
}

func (s *sctpConn) LocalAddr() net.Addr {
	return s.conn.LocalAddr()
}

func (s *sctpConn) RemoteAddr() net.Addr {
	return s.conn.RemoteAddr()
}

func (s *sctpConn) SetDeadline(t time.Time) error {
	return s.conn.SetDeadline(t)
}

func (s *sctpConn) SetWriteDeadline(t time.Time) error {
	return s.conn.SetWriteDeadline(t)
}

func (s *sctpConn) SetReadDeadline(t time.Time) error {
	return s.conn.SetReadDeadline(t)
}
