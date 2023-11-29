package listener

import (
	"github.com/Dreamacro/clash/adapter/inbound"
	"github.com/Dreamacro/clash/config"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/listener/sing_tun"
	"github.com/Dreamacro/clash/log"
	"golang.org/x/exp/slices"
	"sort"
	"sync"
)

var (
	tunLister *sing_tun.Listener

	tunMux sync.Mutex

	LastTunConf config.Tun
)

func GetTunConf() config.Tun {
	if tunLister == nil {
		return LastTunConf
	}
	return tunLister.Config()
}

func ReCreateTun(tunConf config.Tun, tcpIn chan<- C.ConnContext, udpIn chan<- *inbound.PacketAdapter) {
	tunMux.Lock()
	defer func() {
		LastTunConf = tunConf
		tunMux.Unlock()
	}()

	var err error
	defer func() {
		if err != nil {
			log.Errorln("Start TUN listening error: %s", err.Error())
			tunConf.Enable = false
		}
	}()

	if !hasTunConfigChange(&tunConf) {
		if tunLister != nil {
			tunLister.FlushDefaultInterface()
		}
		return
	}

	CloseTunListener()

	if !tunConf.Enable {
		return
	}

	var lister *sing_tun.Listener
	if tunConf.TunIf {
		lister, err = sing_tun.ConnectTun(tunConf, tcpIn, udpIn)
	} else {
		lister, err = sing_tun.New(tunConf, tcpIn, udpIn)
	}
	if err != nil {
		return
	}
	tunLister = lister

	log.Infoln("[TUN] Tun adapter listening at: %s", tunLister.Address())
}

func hasTunConfigChange(tunConf *config.Tun) bool {
	if LastTunConf.Enable != tunConf.Enable ||
		LastTunConf.Device != tunConf.Device ||
		LastTunConf.Stack != tunConf.Stack ||
		LastTunConf.AutoRoute != tunConf.AutoRoute ||
		LastTunConf.AutoDetectInterface != tunConf.AutoDetectInterface ||
		LastTunConf.MTU != tunConf.MTU ||
		LastTunConf.StrictRoute != tunConf.StrictRoute ||
		LastTunConf.EndpointIndependentNat != tunConf.EndpointIndependentNat ||
		LastTunConf.UDPTimeout != tunConf.UDPTimeout ||
		LastTunConf.FileDescriptor != tunConf.FileDescriptor {
		return true
	}

	if len(LastTunConf.DNSHijack) != len(tunConf.DNSHijack) {
		return true
	}

	sort.Slice(tunConf.DNSHijack, func(i, j int) bool {
		return tunConf.DNSHijack[i] < tunConf.DNSHijack[j]
	})

	sort.Slice(tunConf.Inet4Address, func(i, j int) bool {
		return tunConf.Inet4Address[i].String() < tunConf.Inet4Address[j].String()
	})

	sort.Slice(tunConf.Inet6Address, func(i, j int) bool {
		return tunConf.Inet6Address[i].String() < tunConf.Inet6Address[j].String()
	})

	sort.Slice(tunConf.Inet4RouteAddress, func(i, j int) bool {
		return tunConf.Inet4RouteAddress[i].String() < tunConf.Inet4RouteAddress[j].String()
	})

	sort.Slice(tunConf.Inet6RouteAddress, func(i, j int) bool {
		return tunConf.Inet6RouteAddress[i].String() < tunConf.Inet6RouteAddress[j].String()
	})

	sort.Slice(tunConf.IncludeUID, func(i, j int) bool {
		return tunConf.IncludeUID[i] < tunConf.IncludeUID[j]
	})

	sort.Slice(tunConf.IncludeUIDRange, func(i, j int) bool {
		return tunConf.IncludeUIDRange[i] < tunConf.IncludeUIDRange[j]
	})

	sort.Slice(tunConf.ExcludeUID, func(i, j int) bool {
		return tunConf.ExcludeUID[i] < tunConf.ExcludeUID[j]
	})

	sort.Slice(tunConf.ExcludeUIDRange, func(i, j int) bool {
		return tunConf.ExcludeUIDRange[i] < tunConf.ExcludeUIDRange[j]
	})

	sort.Slice(tunConf.IncludeAndroidUser, func(i, j int) bool {
		return tunConf.IncludeAndroidUser[i] < tunConf.IncludeAndroidUser[j]
	})

	sort.Slice(tunConf.IncludePackage, func(i, j int) bool {
		return tunConf.IncludePackage[i] < tunConf.IncludePackage[j]
	})

	sort.Slice(tunConf.ExcludePackage, func(i, j int) bool {
		return tunConf.ExcludePackage[i] < tunConf.ExcludePackage[j]
	})

	if !slices.Equal(tunConf.DNSHijack, LastTunConf.DNSHijack) ||
		!slices.Equal(tunConf.Inet4Address, LastTunConf.Inet4Address) ||
		!slices.Equal(tunConf.Inet6Address, LastTunConf.Inet6Address) ||
		!slices.Equal(tunConf.Inet4RouteAddress, LastTunConf.Inet4RouteAddress) ||
		!slices.Equal(tunConf.Inet6RouteAddress, LastTunConf.Inet6RouteAddress) ||
		!slices.Equal(tunConf.IncludeUID, LastTunConf.IncludeUID) ||
		!slices.Equal(tunConf.IncludeUIDRange, LastTunConf.IncludeUIDRange) ||
		!slices.Equal(tunConf.ExcludeUID, LastTunConf.ExcludeUID) ||
		!slices.Equal(tunConf.ExcludeUIDRange, LastTunConf.ExcludeUIDRange) ||
		!slices.Equal(tunConf.IncludeAndroidUser, LastTunConf.IncludeAndroidUser) ||
		!slices.Equal(tunConf.IncludePackage, LastTunConf.IncludePackage) ||
		!slices.Equal(tunConf.ExcludePackage, LastTunConf.ExcludePackage) {
		return true
	}

	return false
}

func CloseTunListener() {
	if tunLister != nil {
		tunLister.Close()
		tunLister = nil
	}
}
