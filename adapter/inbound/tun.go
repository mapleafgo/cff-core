package inbound

import (
	"net"
	"net/netip"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/context"
)

// NewTunSocket receive TCP inbound and return ConnContext
func NewTunSocket(conn net.Conn, metadata *C.Metadata) *context.ConnContext {
	metadata.NetWork = C.TCP
	metadata.Type = C.TUN
	if ip, port, err := parseAddr(conn.RemoteAddr()); err == nil {
		metadata.SrcIP = ip
		metadata.SrcPort = C.Port(port)
	}
	if addrPort, err := netip.ParseAddrPort(conn.LocalAddr().String()); err == nil {
		metadata.OriginDst = addrPort
	}
	return context.NewConnContext(conn, metadata)
}

// NewTunPacket is PacketAdapter generator
func NewTunPacket(packet C.UDPPacket, metadata *C.Metadata) *PacketAdapter {
	metadata.NetWork = C.UDP
	metadata.Type = C.TUN
	if ip, port, err := parseAddr(packet.LocalAddr()); err == nil {
		metadata.SrcIP = ip
		metadata.SrcPort = C.Port(port)
	}
	return &PacketAdapter{
		UDPPacket: packet,
		metadata:  metadata,
	}
}
