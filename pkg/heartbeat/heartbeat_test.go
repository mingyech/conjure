package heartbeat

import (
	"fmt"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var conf = &Config{Interval: 1 * time.Second, Heartbeat: []byte("hihihihihihihihihi")}

func TestReadWrite(t *testing.T) {
	server, client := net.Pipe()

	s, err := Server(server, conf)
	require.Nil(t, err)

	err = Client(client, conf)
	require.Nil(t, err)

	var wg sync.WaitGroup
	wg.Add(2)

	sent := 0
	recvd := 0
	toSend := []byte("testtt")

	go func() {
		defer wg.Done()
		stop := time.Now().Add(conf.Interval*2 + 10*time.Millisecond)
		fmt.Printf("Now: %v, Stop: %v\n", time.Now(), stop)
		for !time.Now().After(stop) {
			buffer := make([]byte, 4096)
			n, err := s.Read(buffer)
			if err != nil {
				continue
			}
			require.Equal(t, toSend, buffer[:n])
			recvd++
		}
	}()

	go func() {
		defer wg.Done()
		stop := time.Now().Add(conf.Interval * 2)
		for !time.Now().After(stop) {
			_, err := client.Write(toSend)
			require.Nil(t, err)
			sent++
			time.Sleep(10 * time.Millisecond)
		}
	}()

	wg.Wait()
	fmt.Printf("sent: %v, recvd: %v", sent, recvd)

	require.Equal(t, sent, recvd)
}
