package main

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"syscall"
	"unsafe"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func OpenTun(tunName string) (*os.File, error) {
	const (
		IFF_TUN   = 0x0001
		IFF_NO_PI = 0x1000
		TUNSETIFF = 0x400454ca
	)

	tun, err := os.OpenFile("/dev/net/tun", os.O_RDWR, 0)
	if err != nil {
		return nil, err
	}

	var ifreq [0x28]byte
	copy(ifreq[:], tunName)

	flags := IFF_TUN | IFF_NO_PI
	binary.LittleEndian.PutUint16(ifreq[0x10:], uint16(flags))

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, tun.Fd(), uintptr(TUNSETIFF), uintptr(unsafe.Pointer(&ifreq[0])))
	if errno != 0 {
		tun.Close()
		return nil, errno
	}

	return tun, nil
}

func main() {
	// tun, err := os.Open("tunfile")
	// if err != nil {
	// 	fmt.Println(err)
	// 	return
	// }
	tun, err := OpenTun("tun1")
	if err != nil {
		fmt.Println(err)
		return
	}

	src := "1.2.3.5"
	dst := "5.6.7.9"
	sport := 6789

	ipLayer := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		SrcIP:    net.ParseIP(src),
		DstIP:    net.ParseIP(dst),
		Protocol: layers.IPProtocolUDP,
	}

	udpLayer := &layers.UDP{
		SrcPort: layers.UDPPort(sport),
		DstPort: layers.UDPPort(443),
	}
	err = udpLayer.SetNetworkLayerForChecksum(ipLayer)
	if err != nil {
		fmt.Println(err)
		return
	}

	payload := []byte("Hello world")

	buffer := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
	err = gopacket.SerializeLayers(buffer, opts,
		ipLayer,
		udpLayer,
		gopacket.Payload(payload),
	)
	if err != nil {
		panic(err)
	}

	pkt := buffer.Bytes()
	// pkt = []byte{69, 0, 0, 39, 0, 1, 0, 0, 64, 17, 106, 176, 1, 2, 3, 5, 5, 6, 7, 9, 26, 133, 1, 187, 0, 19, 97, 164, 72, 101, 108, 108, 111, 32, 119, 111, 114, 108, 100}
	tun.Write(pkt)

	fmt.Printf("Wrote pkt: %v\n", pkt)
	resp := make([]byte, 1024)
	tun.Read(resp)
	fmt.Println(resp)

	payload = []byte("Hi again")

	buffer = gopacket.NewSerializeBuffer()
	err = gopacket.SerializeLayers(buffer, opts,
		ipLayer,
		udpLayer,
		gopacket.Payload(payload),
	)
	if err != nil {
		panic(err)
	}

	pkt = buffer.Bytes()
	tun.Write(pkt)

	fmt.Println("Wrote again")
	tun.Read(resp)
	fmt.Println(resp)
}
