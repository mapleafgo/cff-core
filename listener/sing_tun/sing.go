package sing_tun

import (
	"context"
	"errors"
	"github.com/Dreamacro/clash/adapter/inbound"
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"

	mux "github.com/sagernet/sing-mux"
	vmess "github.com/sagernet/sing-vmess"
	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/bufio/deadline"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/common/uot"
)

const UDPTimeout = 5 * time.Minute

type ListenerHandler struct {
	TcpIn      chan<- C.ConnContext
	UdpIn      chan<- *inbound.PacketAdapter
	UDPTimeout time.Duration
}

func (h *ListenerHandler) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	if h.IsSpecialFqdn(metadata.Destination.Fqdn) {
		return h.ParseSpecialFqdn(ctx, conn, metadata)
	}

	if deadline.NeedAdditionalReadDeadline(conn) {
		conn = deadline.NewFallbackConn(conn) // conn from sing should check NeedAdditionalReadDeadline
	}

	cMetadata := &C.Metadata{
		NetWork: C.TCP,
		Type:    C.TUN,
	}

	var err error
	cMetadata.DstIP, cMetadata.DstPort, err = SetRemoteAddr(metadata.Destination)
	if err != nil {
		return err
	}
	cMetadata.SrcIP, cMetadata.SrcPort, err = SetRemoteAddr(metadata.Source)
	if err != nil {
		return err
	}

	h.TcpIn <- inbound.NewTunSocket(conn, cMetadata)
	return nil
}

func (h *ListenerHandler) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata M.Metadata) error {
	if deadline.NeedAdditionalReadDeadline(conn) {
		conn = deadline.NewFallbackPacketConn(bufio.NewNetPacketConn(conn)) // conn from sing should check NeedAdditionalReadDeadline
	}
	defer func() { _ = conn.Close() }()
	mutex := sync.Mutex{}
	conn2 := conn // a new interface to set nil in defer
	defer func() {
		mutex.Lock() // this goroutine must exit after all conn.WritePacket() is not running
		defer mutex.Unlock()
		conn2 = nil
	}()
	var buff *buf.Buffer
	newBuffer := func() *buf.Buffer {
		buff = buf.NewPacket() // do not use stack buffer
		return buff
	}
	readWaiter, isReadWaiter := bufio.CreatePacketReadWaiter(conn)
	if isReadWaiter {
		readWaiter.InitializeReadWaiter(newBuffer)
	}
	for {
		var (
			dest M.Socksaddr
			err  error
		)
		buff = nil // clear last loop status, avoid repeat release
		if isReadWaiter {
			dest, err = readWaiter.WaitReadPacket()
		} else {
			dest, err = conn.ReadPacket(newBuffer())
		}
		if err != nil {
			if buff != nil {
				buff.Release()
			}
			if ShouldIgnorePacketError(err) {
				break
			}
			return err
		}
		packet := &packet{
			conn:  &conn2,
			mutex: &mutex,
			rAddr: metadata.Source.UDPAddr(),
			lAddr: conn.LocalAddr(),
			buff:  buff,
		}

		cMetadata := &C.Metadata{
			NetWork: C.UDP,
			Type:    C.TUN,
		}

		cMetadata.DstIP, cMetadata.DstPort, err = SetRemoteAddr(dest)
		if err != nil {
			return err
		}
		cMetadata.SrcIP, cMetadata.SrcPort, err = SetRemoteAddr(metadata.Source)
		if err != nil {
			return err
		}

		select {
		case h.UdpIn <- inbound.NewTunPacket(packet, cMetadata):
		default:
		}
	}
	return nil
}

func (h *ListenerHandler) NewError(ctx context.Context, err error) {
	log.Warnln("TUN listener get error: %+v", err)
}

func UpstreamMetadata(metadata M.Metadata) M.Metadata {
	return M.Metadata{
		Source:      metadata.Source,
		Destination: metadata.Destination,
	}
}

func (h *ListenerHandler) IsSpecialFqdn(fqdn string) bool {
	switch fqdn {
	case mux.Destination.Fqdn:
	case vmess.MuxDestination.Fqdn:
	case uot.MagicAddress:
	case uot.LegacyMagicAddress:
	default:
		return false
	}
	return true
}

func (h *ListenerHandler) ParseSpecialFqdn(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	switch metadata.Destination.Fqdn {
	case mux.Destination.Fqdn:
		return mux.HandleConnection(ctx, h, log.SingLogger, conn, UpstreamMetadata(metadata))
	case vmess.MuxDestination.Fqdn:
		return vmess.HandleMuxConnection(ctx, conn, h)
	case uot.MagicAddress:
		request, err := uot.ReadRequest(conn)
		if err != nil {
			return E.Cause(err, "read UoT request")
		}
		metadata.Destination = request.Destination
		return h.NewPacketConnection(ctx, uot.NewConn(conn, *request), metadata)
	case uot.LegacyMagicAddress:
		metadata.Destination = M.Socksaddr{Addr: netip.IPv4Unspecified()}
		return h.NewPacketConnection(ctx, uot.NewConn(conn, uot.Request{}), metadata)
	}
	return errors.New("not special fqdn")
}

func ShouldIgnorePacketError(err error) bool {
	// ignore simple error
	if E.IsTimeout(err) || E.IsClosed(err) || E.IsCanceled(err) {
		return true
	}
	return false
}

func SetRemoteAddr(addr net.Addr) (net.IP, C.Port, error) {
	if addr == nil {
		return nil, 0, nil
	}
	if rawAddr, ok := addr.(interface{ RawAddr() net.Addr }); ok {
		if rawAddr := rawAddr.RawAddr(); rawAddr != nil {
			if ip, port, err := SetRemoteAddr(rawAddr); err == nil {
				return ip, port, nil
			}
		}
	}
	if addr, ok := addr.(interface{ AddrPort() netip.AddrPort }); ok { // *net.TCPAddr, *net.UDPAddr, M.Socksaddr
		if addrPort := addr.AddrPort(); addrPort.Port() != 0 {
			port := C.Port(addrPort.Port())
			if addrPort.IsValid() { // sing's M.Socksaddr maybe return an invalid AddrPort if it's a DomainName
				return addrPort.Addr().Unmap().AsSlice(), port, nil
			}
		}
	}
	return SetRemoteAddress(addr.String())
}

func SetRemoteAddress(rawAddress string) (net.IP, C.Port, error) {
	host, port, err := net.SplitHostPort(rawAddress)
	if err != nil {
		return nil, 0, err
	}

	var uint16Port C.Port
	if port, err := strconv.ParseUint(port, 10, 16); err == nil {
		uint16Port = C.Port(port)
	}

	if ip, err := netip.ParseAddr(host); err != nil {
		return nil, 0, err
	} else {
		return ip.Unmap().AsSlice(), uint16Port, nil
	}
}

type packet struct {
	conn  *network.PacketConn
	mutex *sync.Mutex
	rAddr net.Addr
	lAddr net.Addr
	buff  *buf.Buffer
}

func (c *packet) Data() []byte {
	return c.buff.Bytes()
}

// WriteBack wirtes UDP packet with source(ip, port) = `addr`
func (c *packet) WriteBack(b []byte, addr net.Addr) (n int, err error) {
	if addr == nil {
		err = errors.New("address is invalid")
		return
	}

	c.mutex.Lock()
	defer c.mutex.Unlock()
	conn := *c.conn
	if conn == nil {
		err = errors.New("writeBack to closed connection")
		return
	}

	buff := buf.NewPacket()
	defer buff.Release()
	n, err = buff.Write(b)
	if err != nil {
		return
	}

	err = conn.WritePacket(buff, M.SocksaddrFromNet(addr))
	if err != nil {
		return
	}
	return
}

// LocalAddr returns the source IP/Port of UDP Packet
func (c *packet) LocalAddr() net.Addr {
	return c.rAddr
}

func (c *packet) Drop() {
	c.buff.Release()
}

func (c *packet) InAddr() net.Addr {
	return c.lAddr
}
