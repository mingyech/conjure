package heartbeat

import "time"

type Config struct {

	// Heartbeat is the payload to sent as the heartbeat
	Heartbeat []byte

	// Interval is the interval of each heartbeat
	Interval time.Duration
}

func validate(c Config) Config {
	if c.Interval == 0 {
		c.Interval = defaultInterval
	}

	if c.Heartbeat == nil {
		c.Heartbeat = defaultHb
	}

	return c
}

var defaultHb = []byte("6v3jyM521GkBo1lsMyVLcRyzdZ7FKEM3")
var defaultInterval = 30 * time.Second
