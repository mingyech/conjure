package dtls

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

var sharedSecret = []byte("hihihihihihihihihihihihihihihihi")

func TestSend(t *testing.T) {
	size := 65535
	toSend := make([]byte, size)

	for i := range toSend {
		toSend[i] = byte('a')
	}

	server, client := net.Pipe()

	go func() {
		s, err := Server(server, sharedSecret)
		require.Nil(t, err)

		received := make([]byte, size)
		_, err = s.Read(received)
		require.Nil(t, err)

		require.Equal(t, toSend, received)
	}()

	c, err := Client(client, sharedSecret)
	require.Nil(t, err)

	_, err = c.Write(toSend)
	require.Nil(t, err)
}
