package route

import (
	"net/http"
	"net/netip"
	"path/filepath"

	"github.com/Dreamacro/clash/component/resolver"
	"github.com/Dreamacro/clash/config"
	C "github.com/Dreamacro/clash/constant"
	"github.com/Dreamacro/clash/hub/executor"
	"github.com/Dreamacro/clash/listener"
	"github.com/Dreamacro/clash/log"
	"github.com/Dreamacro/clash/tunnel"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/render"
	"github.com/samber/lo"
)

func configRouter() http.Handler {
	r := chi.NewRouter()
	r.Get("/", getConfigs)
	r.Put("/", updateConfigs)
	r.Patch("/", patchConfigs)
	return r
}

func getConfigs(w http.ResponseWriter, r *http.Request) {
	general := executor.GetGeneral()
	render.JSON(w, r, general)
}

func patchConfigs(w http.ResponseWriter, r *http.Request) {
	general := struct {
		Port        *int               `json:"port"`
		SocksPort   *int               `json:"socks-port"`
		RedirPort   *int               `json:"redir-port"`
		TProxyPort  *int               `json:"tproxy-port"`
		MixedPort   *int               `json:"mixed-port"`
		AllowLan    *bool              `json:"allow-lan"`
		BindAddress *string            `json:"bind-address"`
		Mode        *tunnel.TunnelMode `json:"mode"`
		LogLevel    *log.LogLevel      `json:"log-level"`
		IPv6        *bool              `json:"ipv6"`
		Tun         *tunSchema         `json:"tun"`
	}{}
	if err := render.DecodeJSON(r.Body, &general); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrBadRequest)
		return
	}

	if general.Mode != nil {
		tunnel.SetMode(*general.Mode)
	}

	if general.LogLevel != nil {
		log.SetLevel(*general.LogLevel)
	}

	if general.IPv6 != nil {
		resolver.DisableIPv6 = !*general.IPv6
	}

	if general.AllowLan != nil {
		listener.SetAllowLan(*general.AllowLan)
	}

	if general.BindAddress != nil {
		listener.SetBindAddress(*general.BindAddress)
	}

	ports := listener.GetPorts()
	ports.Port = lo.FromPtrOr(general.Port, ports.Port)
	ports.SocksPort = lo.FromPtrOr(general.SocksPort, ports.SocksPort)
	ports.RedirPort = lo.FromPtrOr(general.RedirPort, ports.RedirPort)
	ports.TProxyPort = lo.FromPtrOr(general.TProxyPort, ports.TProxyPort)
	ports.MixedPort = lo.FromPtrOr(general.MixedPort, ports.MixedPort)

	listener.ReCreatePortsListeners(*ports, tunnel.TCPIn(), tunnel.UDPIn())
	listener.ReCreateTun(tunFromPtrOr(general.Tun, listener.LastTunConf), tunnel.TCPIn(), tunnel.UDPIn())

	render.NoContent(w, r)
}

func updateConfigs(w http.ResponseWriter, r *http.Request) {
	req := struct {
		Path    string `json:"path"`
		Payload string `json:"payload"`
	}{}
	if err := render.DecodeJSON(r.Body, &req); err != nil {
		render.Status(r, http.StatusBadRequest)
		render.JSON(w, r, ErrBadRequest)
		return
	}

	force := r.URL.Query().Get("force") == "true"
	var cfg *config.Config
	var err error

	if req.Payload != "" {
		cfg, err = executor.ParseWithBytes([]byte(req.Payload))
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError(err.Error()))
			return
		}
	} else {
		if req.Path == "" {
			req.Path = C.Path.Config()
		}
		if !filepath.IsAbs(req.Path) {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError("path is not a absolute path"))
			return
		}

		cfg, err = executor.ParseWithPath(req.Path)
		if err != nil {
			render.Status(r, http.StatusBadRequest)
			render.JSON(w, r, newError(err.Error()))
			return
		}
	}

	executor.ApplyConfig(cfg, force)
	render.NoContent(w, r)
}

type tunSchema struct {
	Enable              bool        `yaml:"enable" json:"enable"`
	Device              *string     `yaml:"device" json:"device"`
	Stack               *C.TUNStack `yaml:"stack" json:"stack"`
	DNSHijack           *[]string   `yaml:"dns-hijack" json:"dns-hijack"`
	AutoRoute           *bool       `yaml:"auto-route" json:"auto-route"`
	AutoDetectInterface *bool       `yaml:"auto-detect-interface" json:"auto-detect-interface"`

	MTU                    *uint32         `yaml:"mtu" json:"mtu,omitempty"`
	Inet6Address           *[]netip.Prefix `yaml:"inet6-address" json:"inet6-address,omitempty"`
	StrictRoute            *bool           `yaml:"strict-route" json:"strict-route,omitempty"`
	Inet4RouteAddress      *[]netip.Prefix `yaml:"inet4-route-address" json:"inet4-route-address,omitempty"`
	Inet6RouteAddress      *[]netip.Prefix `yaml:"inet6-route-address" json:"inet6-route-address,omitempty"`
	IncludeUID             *[]uint32       `yaml:"include-uid" json:"include-uid,omitempty"`
	IncludeUIDRange        *[]string       `yaml:"include-uid-range" json:"include-uid-range,omitempty"`
	ExcludeUID             *[]uint32       `yaml:"exclude-uid" json:"exclude-uid,omitempty"`
	ExcludeUIDRange        *[]string       `yaml:"exclude-uid-range" json:"exclude-uid-range,omitempty"`
	IncludeAndroidUser     *[]int          `yaml:"include-android-user" json:"include-android-user,omitempty"`
	IncludePackage         *[]string       `yaml:"include-package" json:"include-package,omitempty"`
	ExcludePackage         *[]string       `yaml:"exclude-package" json:"exclude-package,omitempty"`
	EndpointIndependentNat *bool           `yaml:"endpoint-independent-nat" json:"endpoint-independent-nat,omitempty"`
	UDPTimeout             *int64          `yaml:"udp-timeout" json:"udp-timeout,omitempty"`
	FileDescriptor         *int            `yaml:"file-descriptor" json:"file-descriptor"`
}

func tunFromPtrOr(p *tunSchema, def config.Tun) config.Tun {
	if p != nil {
		def.Enable = p.Enable
		if p.Device != nil {
			def.Device = *p.Device
		}
		if p.Stack != nil {
			def.Stack = *p.Stack
		}
		if p.DNSHijack != nil {
			def.DNSHijack = *p.DNSHijack
		}
		if p.AutoRoute != nil {
			def.AutoRoute = *p.AutoRoute
		}
		if p.AutoDetectInterface != nil {
			def.AutoDetectInterface = *p.AutoDetectInterface
		}
		if p.MTU != nil {
			def.MTU = *p.MTU
		}
		if p.Inet6Address != nil {
			def.Inet6Address = *p.Inet6Address
		}
		if p.Inet4RouteAddress != nil {
			def.Inet4RouteAddress = *p.Inet4RouteAddress
		}
		if p.Inet6RouteAddress != nil {
			def.Inet6RouteAddress = *p.Inet6RouteAddress
		}
		if p.IncludeUID != nil {
			def.IncludeUID = *p.IncludeUID
		}
		if p.IncludeUIDRange != nil {
			def.IncludeUIDRange = *p.IncludeUIDRange
		}
		if p.ExcludeUID != nil {
			def.ExcludeUID = *p.ExcludeUID
		}
		if p.ExcludeUIDRange != nil {
			def.ExcludeUIDRange = *p.ExcludeUIDRange
		}
		if p.IncludeAndroidUser != nil {
			def.IncludeAndroidUser = *p.IncludeAndroidUser
		}
		if p.IncludePackage != nil {
			def.IncludePackage = *p.IncludePackage
		}
		if p.ExcludePackage != nil {
			def.ExcludePackage = *p.ExcludePackage
		}
		if p.EndpointIndependentNat != nil {
			def.EndpointIndependentNat = *p.EndpointIndependentNat
		}
		if p.UDPTimeout != nil {
			def.UDPTimeout = *p.UDPTimeout
		}
		if p.FileDescriptor != nil {
			def.FileDescriptor = *p.FileDescriptor
		}
	}
	return def
}
