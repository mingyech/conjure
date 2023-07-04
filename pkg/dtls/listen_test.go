package dtls

import (
	"crypto/rand"
	"net"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

var sharedSecret = []byte("hihihihihihihihihihihihihihihihi")

func TestSend(t *testing.T) {
	size := 65535
	toSend := make([]byte, size)

	rand.Read(toSend)

	server, client := net.Pipe()

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		s, err := Server(server, sharedSecret)
		require.Nil(t, err)

		received := make([]byte, size)
		_, err = s.Read(received)
		require.Nil(t, err)

		require.Equal(t, toSend, received)
	}()

	c, err := Client(client, sharedSecret)
	require.Nil(t, err)

	n, err := c.Write(toSend)
	require.Nil(t, err)
	require.Equal(t, len(toSend), n)

	wg.Wait()
}
