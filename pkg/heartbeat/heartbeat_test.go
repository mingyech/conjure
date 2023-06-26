package heartbeat

import (
	"net"
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

	sent := 0
	recvd := 0
	toSend := []byte("testtt")
	stop := time.After(conf.Interval * 2)

	go func() {
		for {
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
		for {
			select {
			case <-stop:
				return
			default:
				_, err := client.Write(toSend)
				require.Nil(t, err)
				sent++
				time.Sleep(10 * time.Millisecond)
			}
		}
	}()

	<-stop

	require.Equal(t, sent, recvd)
}

func TestSend(t *testing.T) {
	server, client := net.Pipe()

	readCh := make(chan []byte)

	go func() {
		for {
			buffer := make([]byte, 4096)
			n, err := server.Read(buffer)
			if err != nil {
				continue
			}

			readCh <- buffer[:n]
		}
	}()

	err := Client(client, conf)
	require.Nil(t, err)

	duration := 2
	stop := time.After(conf.Interval*time.Duration(duration) + 10*time.Millisecond)

	hbCount := 0
	for {
		select {
		case b := <-readCh:
			require.Equal(t, conf.Heartbeat, b)
			hbCount++
		case <-stop:
			require.Equal(t, duration+1, hbCount)
			return
		}
	}

}

func TestTimeout(t *testing.T) {
	server, client := net.Pipe()
	go func() {
		for {
			buffer := make([]byte, 4096)
			client.Read(buffer)
		}
	}()

	s, err := Server(server, conf)
	require.Nil(t, err)

	_, err = s.Write([]byte("123"))
	require.Nil(t, err)

	stop := time.After(conf.Interval + 10*time.Millisecond)
	<-stop
	_, err = s.Write([]byte("123"))
	require.NotNil(t, err)

}
