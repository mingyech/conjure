package transports

import (
	"encoding/binary"
	"net"
	"os"
	"syscall"
	"unsafe"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func NewDNAT(tunName string) (*DNAT, error) {
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

	return &DNAT{
		tun: tun,
	}, nil
}

type DNAT struct {
	tun *os.File
}

func (d *DNAT) AddEntry(src net.IP, sport uint16, dst net.IP, dport uint16) error {
	ipLayer := &layers.IPv4{
		Version:  4,
		IHL:      5,
		TTL:      64,
		SrcIP:    src,
		DstIP:    dst,
		Protocol: layers.IPProtocolUDP,
	}

	udpLayer := &layers.UDP{
		SrcPort: layers.UDPPort(sport),
		DstPort: layers.UDPPort(dport),
	}

	err := udpLayer.SetNetworkLayerForChecksum(ipLayer)
	if err != nil {
		return err
	}

	payload := []byte("Hello world")

	buffer := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}
	gopacket.SerializeLayers(buffer, opts,
		ipLayer,
		udpLayer,
		gopacket.Payload(payload),
	)

	pkt := buffer.Bytes()
	d.tun.Write(pkt)
	return nil
}
