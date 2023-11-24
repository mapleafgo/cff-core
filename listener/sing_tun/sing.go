package sing_tun

import (
	"context"
	"errors"
	"github.com/Dreamacro/clash/adapter/inbound"
	"net"
	"sync"
	"time"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/log"

	"github.com/sagernet/sing/common/buf"
	"github.com/sagernet/sing/common/bufio"
	"github.com/sagernet/sing/common/bufio/deadline"
	E "github.com/sagernet/sing/common/exceptions"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
)

const UDPTimeout = 5 * time.Minute

type ListenerHandler struct {
	TcpIn      chan<- C.ConnContext
	UdpIn      chan<- *inbound.PacketAdapter
	UDPTimeout time.Duration
}

func (h *ListenerHandler) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	if deadline.NeedAdditionalReadDeadline(conn) {
		conn = deadline.NewFallbackConn(conn) // conn from sing should check NeedAdditionalReadDeadline
	}

	h.TcpIn <- inbound.NewTunSocket(conn, metadata.Destination, metadata.Source)
	return nil
}

func (h *ListenerHandler) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata M.Metadata) error {
	if deadline.NeedAdditionalReadDeadline(conn) {
		conn = deadline.NewFallbackPacketConn(bufio.NewNetPacketConn(conn)) // conn from sing should check NeedAdditionalReadDeadline
	}

	c := &PacketCloser{
		packetConn: conn,
		closed:     false,
	}
	defer func() { _ = c.Close() }()

	buffer := buf.NewPacket()
	for {
		buffer.FullReset()
		destination, err := conn.ReadPacket(buffer)
		if err != nil {
			buffer.Release()
			if ShouldIgnorePacketError(err) {
				break
			}
			return err
		}

		packet := &packet{
			packetCloser: c,
			rAddr:        metadata.Source.UDPAddr(),
			lAddr:        conn.LocalAddr(),
			buffer:       *buffer,
		}
		select {
		case h.UdpIn <- inbound.NewTunPacket(packet, destination, metadata.Source):
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

func ShouldIgnorePacketError(err error) bool {
	// ignore simple error
	if E.IsTimeout(err) || E.IsClosed(err) || E.IsCanceled(err) {
		return true
	}
	return false
}

type packet struct {
	packetCloser *PacketCloser
	rAddr        net.Addr
	lAddr        net.Addr
	buffer       buf.Buffer
}

func (c *packet) Data() []byte {
	return c.buffer.Bytes()
}

// WriteBack wirtes UDP packet with source(ip, port) = `addr`
func (c *packet) WriteBack(b []byte, addr net.Addr) (n int, err error) {
	if addr == nil {
		err = errors.New("address is invalid")
		return
	}

	buff := buf.NewPacket()
	defer buff.Release()
	n, err = buff.Write(b)
	if err != nil {
		return
	}

	l := c.packetCloser
	l.Lock()
	defer l.Unlock()
	if l.closed {
		err = errors.New("writeBack to closed connection")
		return
	}

	err = l.packetConn.WritePacket(buff, M.SocksaddrFromNet(addr))
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
	c.buffer.Release()
}

func (c *packet) InAddr() net.Addr {
	return c.lAddr
}

type PacketCloser struct {
	sync.Mutex
	packetConn network.PacketConn
	closed     bool
}

// Close implements C.Listener
func (c *PacketCloser) Close() error {
	c.Lock()
	defer c.Unlock()
	c.closed = true
	return c.packetConn.Close()
}
