package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"sync"

	"github.com/go-redis/redis/v8"
	"github.com/libp2p/go-reuseport"
	"github.com/mingyech/dtls/v2/examples/util"
	"github.com/refraction-networking/conjure/application/transports"
	"github.com/refraction-networking/conjure/pkg/dtls"
	pb "github.com/refraction-networking/gotapdance/protobuf"
	"google.golang.org/protobuf/proto"
)

const DETECTOR_REG_CHANNEL string = "dark_decoy_map"

func main() {
	var remoteAddr = flag.String("raddr", "", "remote address")
	var remoteAddr2 = flag.String("raddr2", "", "remote address 2")
	var localAddr = flag.String("laddr", "", "source address")
	var phantomAddr = flag.String("paddr", "", "phantom address")
	var phantomAddr2 = flag.String("paddr2", "", "phantom address 2")
	var secret = flag.String("secret", "1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef", "shared secret")
	flag.Parse()
	// Prepare the IP to connect to
	// laddr, err := net.ResolveUDPAddr("udp", *localAddr)
	// util.Check(err)

	hub := util.NewHub()

	sharedSecret := []byte(*secret)

	dnat, err := transports.NewDNAT()
	util.Check(err)

	client := getRedisClient()
	if client == nil {
		fmt.Printf("couldn't connect to redis")
		return
	}

	// laddr, err := net.ResolveUDPAddr("udp", *localAddr)
	// util.Check(err)

	// listener, err := dtls.Listen(laddr)
	// util.Check(err)

	fmt.Println("Listening")

	go func() {
		fmt.Println("connecting client 1")
		addr, err := net.ResolveUDPAddr("udp", *remoteAddr)
		util.Check(err)

		paddr, err := net.ResolveUDPAddr("udp", *phantomAddr)
		util.Check(err)

		dnat.AddEntry(addr.IP, uint16(addr.Port), paddr.IP, uint16(paddr.Port))

		msg := &pb.StationToDetector{
			PhantomIp: proto.String(paddr.IP.String()),
			ClientIp:  proto.String(addr.IP.String()),
			DstPort:   proto.Uint32(uint32(paddr.Port)),
			SrcPort:   proto.Uint32(uint32(addr.Port)),
			Proto:     pb.IPProto_Udp.Enum(),
			TimeoutNs: proto.Uint64(60),
			Operation: pb.StationOperations_New.Enum(),
		}

		s2d, err := proto.Marshal(msg)
		util.Check(err)

		client.Publish(context.Background(), DETECTOR_REG_CHANNEL, string(s2d))

		udpConn, err := reuseport.Dial("udp", *localAddr, *remoteAddr)
		util.Check(err)

		dtlsConn, err := dtls.ClientWithContext(context.Background(), udpConn, sharedSecret)
		util.Check(err)

		fmt.Println("new connection")

		hub.Register(dtlsConn)

	}()

	go func() {
		fmt.Println("connecting client 2")
		addr, err := net.ResolveUDPAddr("udp", *remoteAddr2)
		util.Check(err)

		paddr, err := net.ResolveUDPAddr("udp", *phantomAddr2)
		util.Check(err)

		dnat.AddEntry(addr.IP, uint16(addr.Port), paddr.IP, uint16(paddr.Port))

		msg := &pb.StationToDetector{
			PhantomIp: proto.String(paddr.IP.String()),
			ClientIp:  proto.String(addr.IP.String()),
			DstPort:   proto.Uint32(uint32(paddr.Port)),
			SrcPort:   proto.Uint32(uint32(addr.Port)),
			Proto:     pb.IPProto_Udp.Enum(),
			TimeoutNs: proto.Uint64(60),
			Operation: pb.StationOperations_New.Enum(),
		}

		s2d, err := proto.Marshal(msg)
		util.Check(err)

		client.Publish(context.Background(), DETECTOR_REG_CHANNEL, string(s2d))

		udpConn, err := reuseport.Dial("udp", *localAddr, *remoteAddr2)
		util.Check(err)

		dtlsConn, err := dtls.ClientWithContext(context.Background(), udpConn, sharedSecret)
		util.Check(err)

		fmt.Println("new connection")

		hub.Register(dtlsConn)

	}()

	// go func() {
	// 	for {
	// 		// Wait for a connection.
	// 		conn, err := listener.AcceptFromSecret(sharedSecret)
	// 		util.Check(err)

	// 		fmt.Println("new connection")
	// 		// defer conn.Close() // TODO: graceful shutdown

	// 		// `conn` is of type `net.Conn` but may be casted to `dtls.Conn`
	// 		// using `dtlsConn := conn.(*dtls.Conn)` in order to to expose
	// 		// functions like `ConnectionState` etc.

	// 		// Register the connection with the chat hub
	// 		hub.Register(conn)
	// 	}
	// }()

	hub.Chat()

}

var client *redis.Client
var once sync.Once

// Redis client is already multiplexed and long lived. It is threadsafe so it
// should be able to be accessed by multiple registration threads concurrently
// with no issues. PoolSize is tunable in case this ends up being an issue.
func getRedisClient() *redis.Client {
	once.Do(initRedisClient)
	return client
}

func initRedisClient() {
	client = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
		PoolSize: 100,
	})

	ctx := context.Background()
	// Ping to test redis connection
	_, err := client.Ping(ctx).Result()
	util.Check(err)
}
