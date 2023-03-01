module github.com/refraction-networking/conjure

go 1.16

require (
	filippo.io/edwards25519 v1.0.0 // indirect
	git.torproject.org/pluggable-transports/goptlib.git v1.2.0
	github.com/BurntSushi/toml v0.4.1
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/go-redis/redis/v8 v8.11.4
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/golang-lru v0.5.4
	github.com/mingyech/dtls/v2 v2.0.0-20221227014520-4beb8468ab6e
	github.com/mroth/weightedrand v0.4.1
	github.com/pebbe/zmq4 v1.2.7
	github.com/pelletier/go-toml v1.9.4
	github.com/pion/logging v0.2.2
	github.com/pion/sctp v1.8.5
	github.com/refraction-networking/gotapdance v1.3.1
	github.com/sirupsen/logrus v1.9.0
	github.com/stretchr/testify v1.8.1
	gitlab.com/yawning/obfs4.git v0.0.0-20220204003609-77af0cba934d
	golang.org/x/crypto v0.3.0
	google.golang.org/grpc v1.41.0
	google.golang.org/protobuf v1.28.0
)

require github.com/oschwald/geoip2-golang v1.8.0

replace github.com/refraction-networking/gotapdance => github.com/mingyech/gotapdance v1.2.1-0.20221014213106-9c29e74d1bcb
