package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	golog "log"
	"math"
	"math/rand"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	cj "github.com/refraction-networking/conjure/pkg/station/lib"
	"github.com/refraction-networking/conjure/pkg/station/log"
	"github.com/refraction-networking/conjure/pkg/transports"
)

// connManagerConfig
type connManagerConfig struct {
	NewConnDeadline string
	newConnDeadline time.Duration
	TraceDebugRate  int // rate at which to print Debug logging for connections. Rate is computed as 1/n - 0 indicates off.
}

type connManager struct {
	*connStats
	*connManagerConfig
}

func newConnManager(conf *connManagerConfig) *connManager {
	if conf == nil {
		conf = &connManagerConfig{
			NewConnDeadline: "10s",
			newConnDeadline: 10 * time.Second,
			TraceDebugRate:  0,
		}
	}
	return &connManager{&connStats{geoIPMap: make(map[uint]*asnCounts)}, conf}
}

func (cm *connManager) acceptConnections(ctx context.Context, rm *cj.RegistrationManager, logger *log.Logger) {
	// listen for and handle incoming proxy traffic
	listenAddr := &net.TCPAddr{IP: nil, Port: 41245, Zone: ""}
	ln, err := net.ListenTCP("tcp", listenAddr)
	if err != nil {
		logger.Fatalf("failed to listen on %v: %v\n", listenAddr, err)
	}
	defer ln.Close()
	logger.Infof("[STARTUP] Listening on %v\n", ln.Addr())

	for {
		select {
		case <-ctx.Done():
			break
		default:
			newConn, err := ln.AcceptTCP()
			if err != nil {
				logger.Errorf("[ERROR] failed to AcceptTCP on %v: %v\n", ln.Addr(), err)
				continue
			}
			go cm.handleNewConn(rm, newConn)
		}
	}
}

func getOriginalDst(fd uintptr) (net.IP, error) {
	const SockOptOriginalDst = 80
	if sockOpt, err := syscall.GetsockoptIPv6Mreq(int(fd), syscall.IPPROTO_IP, SockOptOriginalDst); err == nil {
		// parse ipv4
		return net.IPv4(sockOpt.Multiaddr[4], sockOpt.Multiaddr[5], sockOpt.Multiaddr[6], sockOpt.Multiaddr[7]), nil
	} else if mtuinfo, err := syscall.GetsockoptIPv6MTUInfo(int(fd), syscall.IPPROTO_IPV6, SockOptOriginalDst); err == nil {
		// parse ipv6
		return net.IP(mtuinfo.Addr.Addr[:]), nil
	} else {
		return nil, err
	}
}

// Handle connection from client
// NOTE: this is called as a goroutine
func (cm *connManager) handleNewConn(regManager *cj.RegistrationManager, clientConn *net.TCPConn) {
	defer clientConn.Close()
	logger := sharedLogger

	fd, err := clientConn.File()
	if err != nil {
		logger.Errorln("failed to get file descriptor on clientConn:", err)
		return
	}

	fdPtr := fd.Fd()
	originalDstIP, err := getOriginalDst(fdPtr)
	if err != nil {
		logger.Errorln("failed to getOriginalDst from fd:", err)
		return
	}

	// We need to set the underlying file descriptor back into
	// non-blocking mode after calling Fd (which puts it into blocking
	// mode), or else deadlines won't work.
	err = syscall.SetNonblock(int(fdPtr), true)
	if err != nil {
		logger.Errorln("failed to set non-blocking mode on fd:", err)
	}
	fd.Close()

	cm.handleNewTCPConn(regManager, clientConn, originalDstIP)
}

func (cm *connManager) handleNewTCPConn(regManager *cj.RegistrationManager, clientConn net.Conn, originalDstIP net.IP) {
	var originalDst, originalSrc string
	if logClientIP {
		originalSrc = clientConn.RemoteAddr().String()
	} else {
		originalSrc = "_"
	}
	originalDst = originalDstIP.String()
	flowDescription := fmt.Sprintf("%s -> %s ", originalSrc, originalDst)
	logger := log.New(os.Stdout, "[CONN] "+flowDescription, golog.Ldate|golog.Lmicroseconds)

	remoteAddr := clientConn.RemoteAddr()
	var remoteIP net.IP
	if addr, ok := remoteAddr.(*net.TCPAddr); ok {
		remoteIP = addr.IP
	} else if addr, ok := remoteAddr.(*net.UDPAddr); ok {
		remoteIP = addr.IP
	} else {
		return
	}
	var asn uint = 0
	var cc string
	var err error
	cc, err = regManager.GeoIP.CC(remoteIP)
	if err != nil {
		logger.Errorln("Failed to get CC:", err)
		return
	}
	if cc != "unk" {
		// logger.Infoln("CC not unk:", cc, "ASN:", asn) // TESTING
		asn, err = regManager.GeoIP.ASN(remoteIP)
		if err != nil {
			logger.Errorln("Failed to get ASN:", err)
			return
		}
	}
	// logger.Infoln("CC:", cc, "ASN:", asn) // TESTING

	count := regManager.CountRegistrations(originalDstIP)
	logger.Debugf("new connection (%d potential registrations)\n", count)
	cj.Stat().AddConn()
	cm.addCreated(asn, cc)

	// Pick random timeout between 5 and 10 seconds, down to millisecond precision
	ms := rand.Int63n(5000) + 5000
	timeout := time.Duration(ms) * time.Millisecond

	// Give the client a deadline to send enough data to identify a transport.
	// This can be reset by transports to give more time for handshakes
	// after a transport is identified.
	deadline := time.Now().Add(timeout)
	err = clientConn.SetDeadline(deadline)
	if err != nil {
		logger.Errorln("error occurred while setting deadline:", err)
	}

	if count < 1 {
		// Here, reading from the connection would be pointless, but
		// since the kernel already ACK'd this connection, we gain no
		// benefit from instantly dropping the connection; the jig is
		// already up. We should keep reading in line with other code paths
		// so the initiator of the connection gains no information
		// about the correctness of their connection.
		//
		// Possible TODO: use NFQUEUE to be able to drop the connection
		// in userspace before the SYN-ACK is sent, increasing probe
		// resistance.
		logger.Debugf("no possible registrations, reading for %v then dropping connection\n", timeout)
		cj.Stat().AddMissedReg()
		cj.Stat().CloseConn()
		cm.createdToDiscard(asn, cc)

		// Copy into io.Discard to keep ACKing until the deadline.
		// This should help prevent fingerprinting; if we let the read
		// buffer fill up and stopped ACKing after 8192 + (buffer size)
		// bytes for obfs4, as an example, that would be quite clear.
		_, err = io.Copy(io.Discard, clientConn)
		if errors.Is(err, syscall.ECONNRESET) {
			// log reset error without client ip
			logger.Errorln("error occurred discarding data (read 0 B): rst")
			cm.discardToReset(asn, cc)
		} else if et, ok := err.(net.Error); ok && et.Timeout() {
			logger.Errorln("error occurred discarding data (read 0 B): timeout")
			cm.discardToTimeout(asn, cc)
		} else if err != nil {
			//Log any other error
			logger.Errorln("error occurred discarding data (read 0 B):", err)
			cm.discardToError(asn, cc)
		} else {
			cm.discardToClose(asn, cc)
		}

		return
	}

	var buf [4096]byte
	received := bytes.Buffer{}
	possibleTransports := regManager.GetWrappingTransports()

	var reg *cj.DecoyRegistration
	var wrapped net.Conn

readLoop:
	for {
		if len(possibleTransports) < 1 {
			logger.Warnf("ran out of possible transports, reading for %v then giving up\n", time.Until(deadline))
			cj.Stat().ConnErr()

			_, err = io.Copy(io.Discard, clientConn)
			if errors.Is(err, syscall.ECONNRESET) {
				// log reset error without client ip
				logger.Errorf("error occurred discarding data (read %d B): rst\n", received.Len())
				cm.discardToReset(asn, cc)
			} else if et, ok := err.(net.Error); ok && et.Timeout() {
				logger.Errorf("error occurred discarding data (read %d B): timeout\n", received.Len())
				cm.discardToTimeout(asn, cc)
			} else if err != nil {
				//Log any other error
				logger.Errorf("error occurred discarding data (read %d B): %v\n", received.Len(), err)
				cm.discardToError(asn, cc)
			} else {
				cm.discardToClose(asn, cc)
			}

			return
		}

		n, err := clientConn.Read(buf[:])
		if err != nil {
			if errors.Is(err, syscall.ECONNRESET) {
				logger.Errorf("got error while reading from connection, giving up after %d bytes: rst\n", received.Len()+n)
				if received.Len() == 0 {
					cm.createdToReset(asn, cc)
				} else {
					cm.readToReset(asn, cc)
				}
			} else if et, ok := err.(net.Error); ok && et.Timeout() {
				logger.Errorf("got error while reading from connection, giving up after %d bytes: timeout\n", received.Len()+n)
				if received.Len() == 0 {
					cm.createdToTimeout(asn, cc)
				} else {
					cm.readToTimeout(asn, cc)
				}
			} else {
				logger.Errorf("got error while reading from connection, giving up after %d bytes: %v\n", received.Len()+n, err)
				if received.Len() == 0 {
					cm.createdToError(asn, cc)
				} else {
					cm.readToError(asn, cc)
				}
			}
			cj.Stat().ConnErr()
			return
		}

		if received.Len() == 0 {
			cm.createdToCheck(asn, cc)
		} else {
			cm.readToCheck(asn, cc)
		}

		received.Write(buf[:n])
		logger.Tracef("read %d bytes so far", received.Len())

	transports:
		for i, t := range possibleTransports {
			reg, wrapped, err = t.WrapConnection(&received, clientConn, originalDstIP, regManager)
			if errors.Is(err, transports.ErrTryAgain) {
				continue transports
			} else if errors.Is(err, transports.ErrNotTransport) {
				logger.Tracef("not transport %s, removing from checks\n", t.Name())
				delete(possibleTransports, i)
				continue transports
			} else if err != nil {
				// If we got here, the error might have been produced while attempting
				// to wrap the connection, which means received and the connection
				// may no longer be valid. We should just give up on this connection.
				d := time.Until(deadline)
				logger.Warnf("got unexpected error from transport %s, sleeping %v then giving up: %v\n", t.Name(), d, err)
				cj.Stat().ConnErr()
				cm.checkToError(asn, cc)
				time.Sleep(d)
				return
			}

			// We found our transport! First order of business: disable deadline
			err = wrapped.SetDeadline(time.Time{})
			if err != nil {
				logger.Errorln("error occurred while setting deadline:", err)
			}

			logger.SetPrefix(fmt.Sprintf("[%s] %s ", t.LogPrefix(), reg.IDString()))
			logger.Debugf("registration found {reg_id: %s, phantom: %s, transport: %s}\n", reg.IDString(), originalDstIP, t.Name())

			regManager.MarkActive(reg)

			cm.checkToFound(asn, cc)
			break readLoop
		}

		if len(possibleTransports) < 1 {
			cm.checkToDiscard(asn, cc)
		} else if received.Len() == 0 {
			cm.checkToCreated(asn, cc)
		} else {
			cm.checkToRead(asn, cc)
		}
	}

	cj.Proxy(reg, wrapped, logger)
	cj.Stat().CloseConn()
}

type statCounts struct {
	// States
	numCreated      int64 // Number of connections that have read 0 bytes so far
	numReading      int64 // Number of connections in the read / read more state trying to find reg that have read at least 1 byte
	numIODiscarding int64 // Number of connections in the io discard state
	numChecking     int64 // Number of connections that have taken a break from reading to check for the wrapping transport

	// Outcomes
	numFound   int64 // Number of connections that found their registration using wrapConnection
	numReset   int64 // Number of connections that received a reset while attempting to find registration
	numTimeout int64 // Number of connections that timed out while attempting to find registration
	numClosed  int64 // Number of connections that closed before finding the associated registration
	numErr     int64 // Number of connections that received an unexpected error

	// Transitions
	numCreatedToDiscard int64 // Number of times connections have moved from Created to Discard
	numCreatedToCheck   int64 // Number of times connections have moved from Created to Check
	numCreatedToReset   int64 // Number of times connections have moved from Created to Reset
	numCreatedToTimeout int64 // Number of times connections have moved from Created to Timeout
	numCreatedToError   int64 // Number of times connections have moved from Created to Error

	numReadToCheck   int64 // Number of times connections have moved from Read to Check
	numReadToTimeout int64 // Number of times connections have moved from Read to Timeout
	numReadToReset   int64 // Number of times connections have moved from Read to Reset
	numReadToError   int64 // Number of times connections have moved from Read to Error

	numCheckToCreated int64 // Number of times connections have moved from Check to Created
	numCheckToRead    int64 // Number of times connections have moved from Check to Read
	numCheckToFound   int64 // Number of times connections have moved from Check to Found
	numCheckToError   int64 // Number of times connections have moved from Check to Error
	numCheckToDiscard int64 // Number of times connections have moved from Check to Discard

	numDiscardToReset   int64 // Number of times connections have moved from Discard to Reset
	numDiscardToTimeout int64 // Number of times connections have moved from Discard to Timeout
	numDiscardToError   int64 // Number of times connections have moved from Discard to Error
	numDiscardToClose   int64 // Number of times connections have moved from Discard to Close

	totalTransitions int64 // Number of all transitions tracked
}

type asnCounts struct {
	cc string
	statCounts
}

type connStats struct {
	m          sync.RWMutex
	epochStart time.Time
	statCounts
	geoIPMap map[uint]*asnCounts
}

func (c *connStats) PrintAndReset(logger *log.Logger) {
	c.m.Lock() // protect both read for print and write for reset.
	defer c.m.Unlock()

	// prevent div by 0 if thread starvation happens
	var epochDur float64 = math.Max(float64(time.Since(c.epochStart).Milliseconds()), 1)

	logger.Infof("conn-stats: %d %d %d %d %d %.3f %d %.3f %d %.3f %d %.3f %d %.3f",
		atomic.LoadInt64(&c.numCreated),
		atomic.LoadInt64(&c.numReading),
		atomic.LoadInt64(&c.numChecking),
		atomic.LoadInt64(&c.numIODiscarding),
		atomic.LoadInt64(&c.numFound),
		1000*float64(atomic.LoadInt64(&c.numFound))/epochDur,
		atomic.LoadInt64(&c.numReset),
		1000*float64(atomic.LoadInt64(&c.numReset))/epochDur,
		atomic.LoadInt64(&c.numTimeout),
		1000*float64(atomic.LoadInt64(&c.numTimeout))/epochDur,
		atomic.LoadInt64(&c.numErr),
		1000*float64(atomic.LoadInt64(&c.numErr))/epochDur,
		atomic.LoadInt64(&c.numClosed),
		1000*float64(atomic.LoadInt64(&c.numClosed))/epochDur,
	)

	for asn, counts := range c.geoIPMap {
		var tt float64 = math.Max(1, float64(atomic.LoadInt64(&counts.totalTransitions)))

		logger.Infof("conn-stats-verbose: %d %s %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %d %.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f %.3f",
			asn,
			counts.cc,
			atomic.LoadInt64(&counts.numCreatedToDiscard),
			atomic.LoadInt64(&counts.numCreatedToCheck),
			atomic.LoadInt64(&counts.numCreatedToReset),
			atomic.LoadInt64(&counts.numCreatedToTimeout),
			atomic.LoadInt64(&counts.numCreatedToError),
			atomic.LoadInt64(&counts.numReadToCheck),
			atomic.LoadInt64(&counts.numReadToTimeout),
			atomic.LoadInt64(&counts.numReadToReset),
			atomic.LoadInt64(&counts.numReadToError),
			atomic.LoadInt64(&counts.numCheckToCreated),
			atomic.LoadInt64(&counts.numCheckToRead),
			atomic.LoadInt64(&counts.numCheckToFound),
			atomic.LoadInt64(&counts.numCheckToError),
			atomic.LoadInt64(&counts.numCheckToDiscard),
			atomic.LoadInt64(&counts.numDiscardToReset),
			atomic.LoadInt64(&counts.numDiscardToTimeout),
			atomic.LoadInt64(&counts.numDiscardToError),
			atomic.LoadInt64(&counts.numDiscardToClose),
			atomic.LoadInt64(&counts.totalTransitions),
			float64(atomic.LoadInt64(&counts.numCreatedToDiscard))/tt,
			float64(atomic.LoadInt64(&counts.numCreatedToCheck))/tt,
			float64(atomic.LoadInt64(&counts.numCreatedToReset))/tt,
			float64(atomic.LoadInt64(&counts.numCreatedToTimeout))/tt,
			float64(atomic.LoadInt64(&counts.numCreatedToError))/tt,
			float64(atomic.LoadInt64(&counts.numReadToCheck))/tt,
			float64(atomic.LoadInt64(&counts.numReadToTimeout))/tt,
			float64(atomic.LoadInt64(&counts.numReadToReset))/tt,
			float64(atomic.LoadInt64(&counts.numReadToError))/tt,
			float64(atomic.LoadInt64(&counts.numCheckToCreated))/tt,
			float64(atomic.LoadInt64(&counts.numCheckToRead))/tt,
			float64(atomic.LoadInt64(&counts.numCheckToFound))/tt,
			float64(atomic.LoadInt64(&counts.numCheckToError))/tt,
			float64(atomic.LoadInt64(&counts.numCheckToDiscard))/tt,
			float64(atomic.LoadInt64(&counts.numDiscardToReset))/tt,
			float64(atomic.LoadInt64(&counts.numDiscardToTimeout))/tt,
			float64(atomic.LoadInt64(&counts.numDiscardToError))/tt,
			float64(atomic.LoadInt64(&counts.numDiscardToClose))/tt,
		)
	}

	c.reset()
}

func (c *connStats) Reset() {
	c.m.Lock()
	defer c.m.Unlock()
	c.reset()
}

func (c *connStats) reset() {
	atomic.StoreInt64(&c.numFound, 0)
	atomic.StoreInt64(&c.numErr, 0)
	atomic.StoreInt64(&c.numTimeout, 0)
	atomic.StoreInt64(&c.numReset, 0)
	atomic.StoreInt64(&c.numClosed, 0)
	atomic.StoreInt64(&c.numCreatedToDiscard, 0)
	atomic.StoreInt64(&c.numCreatedToCheck, 0)
	atomic.StoreInt64(&c.numCreatedToReset, 0)
	atomic.StoreInt64(&c.numCreatedToTimeout, 0)
	atomic.StoreInt64(&c.numCreatedToError, 0)
	atomic.StoreInt64(&c.numReadToCheck, 0)
	atomic.StoreInt64(&c.numReadToTimeout, 0)
	atomic.StoreInt64(&c.numReadToReset, 0)
	atomic.StoreInt64(&c.numReadToError, 0)
	atomic.StoreInt64(&c.numCheckToCreated, 0)
	atomic.StoreInt64(&c.numCheckToRead, 0)
	atomic.StoreInt64(&c.numCheckToFound, 0)
	atomic.StoreInt64(&c.numCheckToError, 0)
	atomic.StoreInt64(&c.numCheckToDiscard, 0)
	atomic.StoreInt64(&c.numDiscardToReset, 0)
	atomic.StoreInt64(&c.numDiscardToTimeout, 0)
	atomic.StoreInt64(&c.numDiscardToError, 0)
	atomic.StoreInt64(&c.numDiscardToClose, 0)
	atomic.StoreInt64(&c.totalTransitions, 0)

	c.geoIPMap = make(map[uint]*asnCounts)

	c.epochStart = time.Now()
}

func (c *connStats) addCreated(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numCreated, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numCreated, 1)
	}
}

func (c *connStats) createdToDiscard(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numCreated, -1)
	atomic.AddInt64(&c.numIODiscarding, 1)
	atomic.AddInt64(&c.numCreatedToDiscard, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numCreated, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numIODiscarding, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numCreatedToDiscard, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) createdToCheck(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numCreated, -1)
	atomic.AddInt64(&c.numChecking, 1)
	atomic.AddInt64(&c.numCreatedToCheck, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numCreated, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numChecking, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numCreatedToCheck, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) createdToReset(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numCreated, -1)
	atomic.AddInt64(&c.numReset, 1)
	atomic.AddInt64(&c.numCreatedToReset, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numCreated, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numReset, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numCreatedToReset, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) createdToTimeout(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numCreated, -1)
	atomic.AddInt64(&c.numTimeout, 1)
	atomic.AddInt64(&c.numCreatedToTimeout, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numCreated, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numTimeout, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numCreatedToTimeout, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) createdToError(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numCreated, -1)
	atomic.AddInt64(&c.numErr, 1)
	atomic.AddInt64(&c.numCreatedToError, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numCreated, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numErr, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numCreatedToError, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) readToCheck(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numReading, -1)
	atomic.AddInt64(&c.numChecking, 1)
	atomic.AddInt64(&c.numReadToCheck, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numReading, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numChecking, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numReadToCheck, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) readToTimeout(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numReading, -1)
	atomic.AddInt64(&c.numTimeout, 1)
	atomic.AddInt64(&c.numReadToTimeout, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numReading, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numTimeout, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numReadToTimeout, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) readToReset(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numReading, -1)
	atomic.AddInt64(&c.numReset, 1)
	atomic.AddInt64(&c.numReadToReset, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numReading, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numReset, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numReadToReset, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) readToError(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numReading, -1)
	atomic.AddInt64(&c.numErr, 1)
	atomic.AddInt64(&c.numReadToError, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numReading, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numErr, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numReadToError, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) checkToCreated(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numChecking, -1)
	atomic.AddInt64(&c.numCreated, 1)
	atomic.AddInt64(&c.numCheckToCreated, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numChecking, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numCreated, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numCheckToCreated, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) checkToRead(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numChecking, -1)
	atomic.AddInt64(&c.numReading, 1)
	atomic.AddInt64(&c.numCheckToRead, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numChecking, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numReading, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numCheckToRead, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) checkToFound(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numChecking, -1)
	atomic.AddInt64(&c.numFound, 1)
	atomic.AddInt64(&c.numCheckToFound, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numChecking, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numFound, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numCheckToFound, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) checkToError(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numChecking, -1)
	atomic.AddInt64(&c.numErr, 1)
	atomic.AddInt64(&c.numCheckToError, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numChecking, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numErr, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numCheckToError, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) checkToDiscard(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numChecking, -1)
	atomic.AddInt64(&c.numIODiscarding, 1)
	atomic.AddInt64(&c.numCheckToDiscard, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numChecking, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numIODiscarding, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numCheckToDiscard, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) discardToReset(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numIODiscarding, -1)
	atomic.AddInt64(&c.numReset, 1)
	atomic.AddInt64(&c.numDiscardToReset, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numIODiscarding, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numReset, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numDiscardToReset, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) discardToTimeout(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numIODiscarding, -1)
	atomic.AddInt64(&c.numTimeout, 1)
	atomic.AddInt64(&c.numDiscardToTimeout, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numIODiscarding, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numTimeout, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numDiscardToTimeout, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) discardToError(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numIODiscarding, -1)
	atomic.AddInt64(&c.numErr, 1)
	atomic.AddInt64(&c.numDiscardToError, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numIODiscarding, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numErr, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numDiscardToError, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func (c *connStats) discardToClose(asn uint, cc string) {
	// Overall tracking
	atomic.AddInt64(&c.numIODiscarding, -1)
	atomic.AddInt64(&c.numClosed, 1)
	atomic.AddInt64(&c.numDiscardToClose, 1)
	atomic.AddInt64(&c.totalTransitions, 1)

	// GeoIP tracking
	if isValidCC(cc) {
		c.m.Lock()
		defer c.m.Unlock()
		if _, ok := c.geoIPMap[asn]; !ok {
			// We haven't seen asn before, so add it to the map
			c.geoIPMap[asn] = &asnCounts{}
			c.geoIPMap[asn].cc = cc
		}
		atomic.AddInt64(&c.geoIPMap[asn].numIODiscarding, -1)
		atomic.AddInt64(&c.geoIPMap[asn].numClosed, 1)
		atomic.AddInt64(&c.geoIPMap[asn].numDiscardToClose, 1)
		atomic.AddInt64(&c.geoIPMap[asn].totalTransitions, 1)
	}
}

func isValidCC(cc string) bool {
	return cc != ""
}
