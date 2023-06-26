module github.com/refraction-networking/conjure

go 1.18

require (
	filippo.io/edwards25519 v1.0.0 // indirect
	git.torproject.org/pluggable-transports/goptlib.git v1.3.0
	github.com/BurntSushi/toml v1.2.1
	github.com/dchest/siphash v1.2.3 // indirect
	github.com/go-redis/redis/v8 v8.11.5
	github.com/gorilla/mux v1.8.0
	github.com/hashicorp/golang-lru v0.6.0
	github.com/mingyech/dtls/v2 v2.0.0-20230612212344-55a590c00078
	github.com/mroth/weightedrand v1.0.0
	github.com/pebbe/zmq4 v1.2.9
	github.com/pelletier/go-toml v1.9.5
	github.com/pion/logging v0.2.2
	github.com/pion/sctp v1.8.5
	github.com/refraction-networking/gotapdance v1.5.2
	github.com/sirupsen/logrus v1.9.0
	github.com/stretchr/testify v1.8.4
	gitlab.com/yawning/obfs4.git v0.0.0-20220904064028-336a71d6e4cf
	golang.org/x/crypto v0.9.0
	google.golang.org/grpc v1.52.0
	google.golang.org/protobuf v1.30.0
)

require github.com/oschwald/geoip2-golang v1.8.0

require (
	github.com/mingyech/transport/v2 v2.0.0-20230612212211-3c84742e27c5 // indirect
	github.com/pion/dtls/v2 v2.2.7 // indirect
	github.com/pion/transport/v2 v2.2.1 // indirect
)

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/flynn/noise v1.0.0 // indirect
	github.com/google/gopacket v1.1.19
	github.com/libp2p/go-reuseport v0.3.0
	github.com/oschwald/maxminddb-golang v1.10.0 // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/stun v0.6.0
	github.com/pmezard/go-difflib v1.0.0 // indirect
	gitlab.com/yawning/edwards25519-extra.git v0.0.0-20220726154925-def713fd18e4 // indirect
	golang.org/x/sys v0.8.0
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/refraction-networking/gotapdance => github.com/mingyech/gotapdance v1.2.1-0.20230622085827-d6d77d4a8a06

replace gitlab.com/yawning/obfs4.git => github.com/jmwample/obfs4 v0.0.0-20230113193642-07b111e6b208
