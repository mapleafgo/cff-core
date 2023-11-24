package inbound

import (
	"net"
	"net/netip"
	"strconv"

	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/context"
)

// NewTunSocket receive TCP inbound and return ConnContext
func NewTunSocket(conn net.Conn, addr net.Addr, source net.Addr) *context.ConnContext {
	cMetadata := &C.Metadata{
		NetWork: C.TCP,
		Type:    C.TUN,
	}
	if ip, port, err := parseTunAddr(addr); err == nil {
		cMetadata.DstIP = ip
		cMetadata.DstPort = port
	}
	if ip, port, err := parseTunAddr(source); err == nil {
		cMetadata.SrcIP = ip
		cMetadata.SrcPort = port
	}
	if addrPort, err := netip.ParseAddrPort(conn.LocalAddr().String()); err == nil {
		cMetadata.OriginDst = addrPort
	}
	return context.NewConnContext(conn, cMetadata)
}

// NewTunPacket is PacketAdapter generator
func NewTunPacket(packet C.UDPPacket, addr net.Addr, source net.Addr) *PacketAdapter {
	cMetadata := &C.Metadata{
		NetWork: C.UDP,
		Type:    C.TUN,
	}
	if ip, port, err := parseTunAddr(addr); err == nil {
		cMetadata.DstIP = ip
		cMetadata.DstPort = port
	}
	if ip, port, err := parseTunAddr(source); err == nil {
		cMetadata.SrcIP = ip
		cMetadata.SrcPort = port
	}
	return &PacketAdapter{
		UDPPacket: packet,
		metadata:  cMetadata,
	}
}

func parseTunAddr(addr net.Addr) (net.IP, C.Port, error) {
	if addr == nil {
		return nil, 0, nil
	}
	if rawAddr, ok := addr.(interface{ RawAddr() net.Addr }); ok {
		if rawAddr := rawAddr.RawAddr(); rawAddr != nil {
			if ip, port, err := parseTunAddr(rawAddr); err == nil {
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
	return parseTunAddress(addr.String())
}

func parseTunAddress(rawAddress string) (net.IP, C.Port, error) {
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
