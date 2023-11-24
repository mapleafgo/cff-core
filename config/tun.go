package config

import (
	"net"
	"net/netip"

	C "github.com/Dreamacro/clash/constant"
)

type RawTun struct {
	Enable              bool       `yaml:"enable" json:"enable"`
	Device              string     `yaml:"device" json:"device"`
	Stack               C.TUNStack `yaml:"stack" json:"stack"`
	DNSHijack           []string   `yaml:"dns-hijack" json:"dns-hijack"`
	AutoRoute           bool       `yaml:"auto-route" json:"auto-route"`
	AutoDetectInterface bool       `yaml:"auto-detect-interface"`
	RedirectToTun       []string   `yaml:"-" json:"-"`

	MTU uint32 `yaml:"mtu" json:"mtu,omitempty"`
	//Inet4Address           []netip.Prefix `yaml:"inet4-address" json:"inet4_address,omitempty"`
	Inet6Address             []netip.Prefix `yaml:"inet6-address" json:"inet6_address,omitempty"`
	StrictRoute              bool           `yaml:"strict-route" json:"strict_route,omitempty"`
	Inet4RouteAddress        []netip.Prefix `yaml:"inet4-route-address" json:"inet4_route_address,omitempty"`
	Inet6RouteAddress        []netip.Prefix `yaml:"inet6-route-address" json:"inet6_route_address,omitempty"`
	Inet4RouteExcludeAddress []netip.Prefix `yaml:"inet4-route-exclude-address" json:"inet4_route_exclude_address,omitempty"`
	Inet6RouteExcludeAddress []netip.Prefix `yaml:"inet6-route-exclude-address" json:"inet6_route_exclude_address,omitempty"`
	IncludeUID               []uint32       `yaml:"include-uid" json:"include_uid,omitempty"`
	IncludeUIDRange          []string       `yaml:"include-uid-range" json:"include_uid_range,omitempty"`
	ExcludeUID               []uint32       `yaml:"exclude-uid" json:"exclude_uid,omitempty"`
	ExcludeUIDRange          []string       `yaml:"exclude-uid-range" json:"exclude_uid_range,omitempty"`
	IncludeAndroidUser       []int          `yaml:"include-android-user" json:"include_android_user,omitempty"`
	IncludePackage           []string       `yaml:"include-package" json:"include_package,omitempty"`
	ExcludePackage           []string       `yaml:"exclude-package" json:"exclude_package,omitempty"`
	EndpointIndependentNat   bool           `yaml:"endpoint-independent-nat" json:"endpoint_independent_nat,omitempty"`
	UDPTimeout               int64          `yaml:"udp-timeout" json:"udp_timeout,omitempty"`
	FileDescriptor           int            `yaml:"file-descriptor" json:"file-descriptor"`
}

type Tun struct {
	Enable              bool       `yaml:"enable" json:"enable"`
	Device              string     `yaml:"device" json:"device"`
	Stack               C.TUNStack `yaml:"stack" json:"stack"`
	DNSHijack           []string   `yaml:"dns-hijack" json:"dns-hijack"`
	AutoRoute           bool       `yaml:"auto-route" json:"auto-route"`
	AutoDetectInterface bool       `yaml:"auto-detect-interface" json:"auto-detect-interface"`

	MTU                    uint32         `yaml:"mtu" json:"mtu,omitempty"`
	Inet4Address           []netip.Prefix `yaml:"inet4-address" json:"inet4-address,omitempty"`
	Inet6Address           []netip.Prefix `yaml:"inet6-address" json:"inet6-address,omitempty"`
	StrictRoute            bool           `yaml:"strict-route" json:"strict-route,omitempty"`
	Inet4RouteAddress      []netip.Prefix `yaml:"inet4-route-address" json:"inet4-route-address,omitempty"`
	Inet6RouteAddress      []netip.Prefix `yaml:"inet6-route-address" json:"inet6-route-address,omitempty"`
	IncludeUID             []uint32       `yaml:"include-uid" json:"include-uid,omitempty"`
	IncludeUIDRange        []string       `yaml:"include-uid-range" json:"include-uid-range,omitempty"`
	ExcludeUID             []uint32       `yaml:"exclude-uid" json:"exclude-uid,omitempty"`
	ExcludeUIDRange        []string       `yaml:"exclude-uid-range" json:"exclude-uid-range,omitempty"`
	IncludeAndroidUser     []int          `yaml:"include-android-user" json:"include-android-user,omitempty"`
	IncludePackage         []string       `yaml:"include-package" json:"include-package,omitempty"`
	ExcludePackage         []string       `yaml:"exclude-package" json:"exclude-package,omitempty"`
	EndpointIndependentNat bool           `yaml:"endpoint-independent-nat" json:"endpoint-independent-nat,omitempty"`
	UDPTimeout             int64          `yaml:"udp-timeout" json:"udp-timeout,omitempty"`
	FileDescriptor         int            `yaml:"file-descriptor" json:"file-descriptor"`
}

func defaultTun() *RawTun {
	return &RawTun{
		Enable:              false,
		Device:              "",
		Stack:               C.TunGvisor,
		DNSHijack:           []string{"0.0.0.0:53"}, // default hijack all dns query
		AutoRoute:           true,
		AutoDetectInterface: true,
		Inet6Address:        []netip.Prefix{netip.MustParsePrefix("fdfe:dcba:9876::1/126")},
	}
}

func parseTun(rawTun RawTun, general *General, dns *DNS) error {
	var tunAddress netip.Prefix
	pool := dns.FakeIPRange
	if pool != nil {
		g := pool.Gateway()
		ip, _ := netip.AddrFromSlice(g)
		maskSize, _ := pool.IPNet().Mask.Size()
		tunAddress = netip.PrefixFrom(ip, maskSize)
	}
	if !tunAddress.IsValid() {
		tunAddress = netip.MustParsePrefix("198.18.0.1/16")
	}
	tunAddress = netip.PrefixFrom(tunAddress.Addr(), 30)

	if !general.IPv6 || !verifyIP6() {
		rawTun.Inet6Address = nil
	}

	general.Tun = Tun{
		Enable:              rawTun.Enable,
		Device:              rawTun.Device,
		Stack:               rawTun.Stack,
		DNSHijack:           rawTun.DNSHijack,
		AutoRoute:           rawTun.AutoRoute,
		AutoDetectInterface: rawTun.AutoDetectInterface,

		MTU:                    rawTun.MTU,
		Inet4Address:           []netip.Prefix{tunAddress},
		Inet6Address:           rawTun.Inet6Address,
		StrictRoute:            rawTun.StrictRoute,
		Inet4RouteAddress:      rawTun.Inet4RouteAddress,
		Inet6RouteAddress:      rawTun.Inet6RouteAddress,
		IncludeUID:             rawTun.IncludeUID,
		IncludeUIDRange:        rawTun.IncludeUIDRange,
		ExcludeUID:             rawTun.ExcludeUID,
		ExcludeUIDRange:        rawTun.ExcludeUIDRange,
		IncludeAndroidUser:     rawTun.IncludeAndroidUser,
		IncludePackage:         rawTun.IncludePackage,
		ExcludePackage:         rawTun.ExcludePackage,
		EndpointIndependentNat: rawTun.EndpointIndependentNat,
		UDPTimeout:             rawTun.UDPTimeout,
		FileDescriptor:         rawTun.FileDescriptor,
	}

	return nil
}

func verifyIP6() bool {
	if iAddrs, err := net.InterfaceAddrs(); err == nil {
		for _, addr := range iAddrs {
			if prefix, err := netip.ParsePrefix(addr.String()); err == nil {
				if addr := prefix.Addr().Unmap(); addr.Is6() && addr.IsGlobalUnicast() {
					return true
				}
			}
		}
	}
	return false
}
