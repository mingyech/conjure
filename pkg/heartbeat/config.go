package heartbeat

import "time"

type Config struct {

	// Heartbeat is the payload to sent as the heartbeat
	Heartbeat []byte

	// Interval is the checking interval of each heartbeat
	Interval time.Duration
}

func validate(conf *Config) Config {
	if conf == nil {
		return defaultConfig
	}
	c := *conf

	if c.Interval == 0 {
		c.Interval = defaultConfig.Interval
	}

	if c.Heartbeat == nil {
		c.Heartbeat = defaultConfig.Heartbeat
	}

	return c
}

var defaultConfig = Config{
	Heartbeat: []byte("6v3jyM521GkBo1lsMyVLcRyzdZ7FKEM3"),
	Interval:  30 * time.Second,
}
