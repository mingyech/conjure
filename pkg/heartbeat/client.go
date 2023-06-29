package heartbeat

import (
	"net"
	"time"
)

// Client sends heartbeats over conn with config
func Client(conn net.Conn, config *Config) error {
	conf := validate(config)
	go func() {
		for {
			_, err := conn.Write(conf.Heartbeat)
			if err != nil {
				return
			}

			time.Sleep(conf.Interval / 2)
		}

	}()
	return nil
}
