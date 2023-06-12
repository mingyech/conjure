package transports

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strconv"
	"syscall"
	"unsafe"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func NewDNAT() (*DNAT, error) {
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

	coreCountStr := os.Getenv("CJ_CORECOUNT")
	coreCount, err := strconv.Atoi(coreCountStr)
	if err != nil {

		return nil, fmt.Errorf("error parsing core count: %v", err)
	}

	offsetStr := os.Getenv("OFFSET")
	offset, err := strconv.Atoi(offsetStr)
	if err != nil {
		return nil, fmt.Errorf("error parsing offset: %v", err)
	}

	copy(ifreq[:], "tun"+strconv.Itoa(offset+coreCount))

	flags := IFF_TUN | IFF_NO_PI
	binary.LittleEndian.PutUint16(ifreq[0x10:], uint16(flags))

	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, tun.Fd(), uintptr(TUNSETIFF), uintptr(unsafe.Pointer(&ifreq[0])))
	if errno != 0 {
		tun.Close()
		return nil, errno
	}

	// Get the interface name
	name := string(ifreq[:bytes.IndexByte(ifreq[:], 0)])

	fmt.Println("Interface Name:", name)

	// Bring the interface up
	err = setUp(tun, name)
	if err != nil {
		return nil, fmt.Errorf("error bring the interface up: %v", err)
	}

	return &DNAT{
		tun: tun,
	}, nil
}

// setUp brings up a network interface represented by the given name.
func setUp(tun *os.File, name string) error {
	ifreq := make([]byte, 0x28)

	// Populate the interface name
	copy(ifreq[:], name)

	// Get the current interface flags
	_, _, errno := syscall.Syscall(syscall.SYS_IOCTL, tun.Fd(), uintptr(syscall.SIOCGIFFLAGS), uintptr(unsafe.Pointer(&ifreq[0])))
	if errno != 0 {
		tun.Close()
		return fmt.Errorf("error getting interface flags: %v", errno)
	}

	// Add the IFF_UP flag to bring the interface up
	flags := binary.LittleEndian.Uint16(ifreq[0x10:])
	flags |= syscall.IFF_UP
	binary.LittleEndian.PutUint16(ifreq[0x10:], flags)

	// Set the new interface flags
	_, _, errno = syscall.Syscall(syscall.SYS_IOCTL, tun.Fd(), uintptr(syscall.SIOCSIFFLAGS), uintptr(unsafe.Pointer(&ifreq[0])))
	if errno != 0 {
		tun.Close()
		return fmt.Errorf("error setting interface flags: %v", errno)
	}

	return nil
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
