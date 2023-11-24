package sing_tun

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"net/netip"
	"time"

	"github.com/Dreamacro/clash/common/pool"
	"github.com/Dreamacro/clash/component/resolver"
	"github.com/Dreamacro/clash/log"
	D "github.com/miekg/dns"

	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
)

const DefaultDnsReadTimeout = time.Second * 10

type DnsListenerHandler struct {
	ListenerHandler
	DnsAdds []netip.AddrPort
}

func (h *DnsListenerHandler) NewError(ctx context.Context, err error) {
	log.Warnln("TUN DNS udpCloser get error: %+v", err)
}

func (h *DnsListenerHandler) ShouldHijackDns(targetAddr netip.AddrPort) bool {
	if targetAddr.Addr().IsLoopback() && targetAddr.Port() == 53 { // cause by system stack
		return true
	}
	for _, addrPort := range h.DnsAdds {
		if addrPort == targetAddr || (addrPort.Addr().IsUnspecified() && targetAddr.Port() == 53) {
			return true
		}
	}
	return false
}

func (h *DnsListenerHandler) NewConnection(ctx context.Context, conn net.Conn, metadata M.Metadata) error {
	if h.ShouldHijackDns(metadata.Destination.AddrPort()) {
		log.Debugln("[DNS] hijack tcp:%s", metadata.Destination.String())
		buff := pool.Get(pool.UDPBufferSize)
		defer func() {
			_ = pool.Put(buff)
			_ = conn.Close()
		}()
		for {
			if conn.SetReadDeadline(time.Now().Add(DefaultDnsReadTimeout)) != nil {
				break
			}

			length := uint16(0)
			if err := binary.Read(conn, binary.BigEndian, &length); err != nil {
				break
			}

			if int(length) > len(buff) {
				break
			}

			n, err := io.ReadFull(conn, buff[:length])
			if err != nil {
				break
			}

			err = func() error {
				inData := buff[:n]
				msg, err := RelayDnsPacket(inData)
				if err != nil {
					return err
				}

				err = binary.Write(conn, binary.BigEndian, uint16(len(msg)))
				if err != nil {
					return err
				}

				_, err = conn.Write(msg)
				if err != nil {
					return err
				}
				return nil
			}()
			if err != nil {
				return err
			}
		}
		return nil
	}
	return h.ListenerHandler.NewConnection(ctx, conn, metadata)
}

func (h *DnsListenerHandler) NewPacketConnection(ctx context.Context, conn network.PacketConn, metadata M.Metadata) error {
	if h.ShouldHijackDns(metadata.Destination.AddrPort()) {
		log.Debugln("[DNS] hijack udp:%s from %s", metadata.Destination.String(), metadata.Source.String())

		c := &PacketCloser{
			packetConn: conn,
			closed:     false,
		}
		defer func() { _ = c.Close() }()

		// safe size which is 1232 from https://dnsflagday.net/2020/.
		// so 2048 is enough
		buff := buf.NewSize(2 * 1024)
		for {
			_ = conn.SetReadDeadline(time.Now().Add(DefaultDnsReadTimeout))
			buff.FullReset()
			dest, err := conn.ReadPacket(buff)
			if err != nil {
				if buff != nil {
					buff.Release()
				}
				if ShouldIgnorePacketError(err) {
					break
				}
				return err
			}
			go func(buffer buf.Buffer) {
				inData := buffer.Bytes()
				msg, err := RelayDnsPacket(inData)
				if err != nil {
					buffer.Release()
					return
				}
				buffer.Reset()
				_, err = buffer.Write(msg)
				if err != nil {
					buffer.Release()
					return
				}
				c.Lock()
				defer c.Unlock()
				if c.closed {
					return
				}
				err = c.packetConn.WritePacket(&buffer, dest) // WritePacket will release buffer
				if err != nil {
					return
				}
			}(*buff) // catch buffer at goroutine create, avoid next loop change buffer
		}
		return nil
	}
	return h.ListenerHandler.NewPacketConnection(ctx, conn, metadata)
}

func RelayDnsPacket(payload []byte) ([]byte, error) {
	msg := &D.Msg{}
	if err := msg.Unpack(payload); err != nil {
		return nil, err
	}

	r, err := resolver.ServeMsg(msg)
	if err != nil {
		m := new(D.Msg)
		m.SetRcode(msg, D.RcodeServerFailure)
		return m.Pack()
	}

	r.SetRcode(msg, r.Rcode)
	r.Compress = true
	return r.Pack()
}
