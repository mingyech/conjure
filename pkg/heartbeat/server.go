package heartbeat

import (
	"bytes"
	"net"
	"sync/atomic"
	"time"
)

type serverConn struct {
	conn    net.Conn
	recvCh  chan errBytes
	waiting uint32
	hb      []byte
	timeout time.Duration
}

type errBytes struct {
	b   []byte
	err error
}

// Server listens for heartbeat over conn with config
func Server(conn net.Conn, config *Config) (net.Conn, error) {
	conf := validate(*config)

	c := &serverConn{conn: conn,
		recvCh:  make(chan errBytes),
		timeout: conf.Interval,
		hb:      conf.Heartbeat,
	}

	atomic.StoreUint32(&c.waiting, 2)

	go c.recvLoop()
	go c.hbLoop()

	return c, nil
}

func (c *serverConn) hbLoop() {
	for {
		if atomic.LoadUint32(&c.waiting) == 0 {
			c.conn.Close()
			return
		}

		atomic.StoreUint32(&c.waiting, 0)
		time.Sleep(c.timeout)
	}

}

func (c *serverConn) recvLoop() {
	for {
		// create a buffer to hold your data
		buffer := make([]byte, 2048)

		n, err := c.conn.Read(buffer)

		if bytes.Equal(c.hb, buffer[:n]) {
			atomic.AddUint32(&c.waiting, 1)
			continue
		}

		c.recvCh <- errBytes{buffer[:n], err}
	}

}

func (c *serverConn) Close() error {
	return c.conn.Close()
}

func (c *serverConn) Write(b []byte) (n int, err error) {
	return c.conn.Write(b)
}

func (c *serverConn) Read(b []byte) (n int, err error) {
	readBytes := <-c.recvCh
	copy(b, readBytes.b)

	return len(readBytes.b), readBytes.err
}

func (c *serverConn) LocalAddr() net.Addr {
	return c.conn.LocalAddr()
}

func (c *serverConn) RemoteAddr() net.Addr {
	return c.conn.RemoteAddr()
}

func (c *serverConn) SetDeadline(t time.Time) error {
	return c.conn.SetDeadline(t)
}

func (c *serverConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *serverConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
