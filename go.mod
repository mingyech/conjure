module github.com/refraction-networking/conjure

go 1.18

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

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/flynn/noise v1.0.0 // indirect
	github.com/oschwald/maxminddb-golang v1.10.0 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/transport v0.14.1 // indirect
	github.com/pion/udp v0.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gitlab.com/yawning/edwards25519-extra.git v0.0.0-20211229043746-2f91fcc9fbdb // indirect
	golang.org/x/sys v0.2.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/refraction-networking/gotapdance => github.com/mingyech/gotapdance v1.2.1-0.20221014213106-9c29e74d1bcb
