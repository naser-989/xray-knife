package singbox

import (
	"context"
	"errors"
	"fmt"
	"github.com/fatih/color"
	"github.com/naser-989/xray-knife/v3/pkg/protocol"
	"github.com/naser-989/xray-knife/v3/utils"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/outbound"
	"github.com/sagernet/sing/common/logger"
	"github.com/xtls/xray-core/infra/conf"
	"net"
	"net/url"
	"reflect"
	"strconv"
	"strings"
)

func NewTrojan(link string) Protocol {
	return &Trojan{OrigLink: link}
}

func (t *Trojan) Name() string {
	return "trojan"
}

func (t *Trojan) Parse() error {
	if !strings.HasPrefix(t.OrigLink, protocol.TrojanIdentifier) {
		return fmt.Errorf("trojan unreconized: %s", t.OrigLink)
	}
	uri, err := url.Parse(t.OrigLink)
	if err != nil {
		return err
	}

	t.Password = uri.User.String()
	t.Address, t.Port, err = net.SplitHostPort(uri.Host)
	if err != nil {
		return err
	}

	if utils.IsIPv6(t.Address) {
		t.Address = "[" + t.Address + "]"
	}
	// Get the type of the struct
	structType := reflect.TypeOf(*t)

	// Get the number of fields in the struct
	numFields := structType.NumField()

	// Iterate over each field of the struct
	for i := 0; i < numFields; i++ {
		field := structType.Field(i)
		tag := field.Tag.Get("json")

		// If the query value exists for the field, set it
		if values, ok := uri.Query()[tag]; ok {
			value := values[0]
			v := reflect.ValueOf(t).Elem().FieldByName(field.Name)

			switch v.Type().String() {
			case "string":
				v.SetString(value)
			case "int":
				var intValue int
				fmt.Sscanf(value, "%d", &intValue)
				v.SetInt(int64(intValue))
			}
		}
	}

	t.Remark, err = url.PathUnescape(uri.Fragment)
	if err != nil {
		t.Remark = uri.Fragment
	}

	if t.HeaderType == "http" || t.Type == "ws" || t.Type == "h2" {
		if t.Path == "" {
			t.Path = "/"
		}
	}

	if t.Type == "" {
		t.Type = "tcp"
	}
	if t.Security == "" {
		t.Security = "tls"
	}
	if t.TlsFingerprint == "" {
		t.TlsFingerprint = "chrome"
	}

	return nil
}

func (t *Trojan) DetailsStr() string {
	copyV := *t
	if copyV.Flow == "" || copyV.Type == "grpc" {
		copyV.Flow = "none"
	}
	info := fmt.Sprintf("%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %v\n%s: %s\n%s: %s\n",
		color.RedString("Protocol"), t.Name(),
		color.RedString("Remark"), t.Remark,
		color.RedString("Network"), t.Type,
		color.RedString("Address"), t.Address,
		color.RedString("Port"), t.Port,
		color.RedString("Password"), t.Password,
		color.RedString("Flow"), copyV.Flow,
	)

	if copyV.Type == "" {

	} else if copyV.Type == "http" || copyV.Type == "httpupgrade" || copyV.Type == "ws" || copyV.Type == "h2" || copyV.Type == "splithttp" {
		info += fmt.Sprintf("%s: %s\n%s: %s\n",
			color.RedString("Host"), copyV.Host,
			color.RedString("Path"), copyV.Path)
	} else if copyV.Type == "kcp" {
		info += fmt.Sprintf("%s: %s\n", color.RedString("KCP Seed"), copyV.Path)
	} else if copyV.Type == "grpc" {
		if copyV.ServiceName == "" {
			copyV.ServiceName = "none"
		}
		info += fmt.Sprintf("%s: %s\n", color.RedString("ServiceName"), copyV.ServiceName)
	}

	if copyV.Security == "reality" {
		info += fmt.Sprintf("%s: reality\n", color.RedString("TLS"))
		if copyV.SpiderX == "" {
			copyV.SpiderX = "none"
		}
		info += fmt.Sprintf("%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %s\n",
			color.RedString("Public key"), copyV.PublicKey,
			color.RedString("SNI"), copyV.SNI,
			color.RedString("ShortID"), copyV.ShortIds,
			color.RedString("SpiderX"), copyV.SpiderX,
			color.RedString("Fingerprint"), copyV.TlsFingerprint,
		)
	} else if copyV.Security == "tls" {
		info += fmt.Sprintf("%s: tls\n", color.RedString("TLS"))
		if len(copyV.SNI) == 0 {
			if copyV.Host != "" {
				copyV.SNI = copyV.Host
			} else {
				copyV.SNI = "none"
			}
		}
		if len(copyV.ALPN) == 0 {
			copyV.ALPN = "none"
		}
		if copyV.TlsFingerprint == "" {
			copyV.TlsFingerprint = "none"
		}
		info += fmt.Sprintf("%s: %s\n%s: %s\n%s: %s\n",
			color.RedString("SNI"), copyV.SNI,
			color.RedString("ALPN"), copyV.ALPN,
			color.RedString("Fingerprint"), copyV.TlsFingerprint)

		if t.AllowInsecure != "" {
			info += fmt.Sprintf("%s: %v\n",
				color.RedString("Insecure"), t.AllowInsecure)
		}
	} else {
		info += fmt.Sprintf("%s: none\n", color.RedString("TLS"))
	}
	return info
}

func (t *Trojan) ConvertToGeneralConfig() (g protocol.GeneralConfig) {
	g.Protocol = t.Name()
	g.Address = t.Address
	g.Host = t.Host
	g.ID = t.Password
	g.Path = t.Path
	g.Port = t.Port
	g.Remark = t.Remark
	g.SNI = t.SNI
	g.ALPN = t.ALPN
	if t.Security == "" {
		g.TLS = "none"
	} else {
		g.TLS = t.Security
	}
	g.TlsFingerprint = t.TlsFingerprint
	g.ServiceName = t.ServiceName
	g.Mode = t.Mode
	g.Type = t.Type
	g.OrigLink = t.OrigLink

	return g
}

func (t *Trojan) CraftInboundOptions() *option.Inbound {
	return &option.Inbound{
		Type: t.Name(),
	}
}

func (t *Trojan) CraftOutboundOptions(allowInsecure bool) (*option.Outbound, error) {
	port, _ := strconv.Atoi(t.Port)

	tls := false
	var alpn []string
	var fingerprint string
	var insecure = allowInsecure

	if t.Security == "tls" || t.Security == "reality" {
		tls = true

		alpn = []string{"http/1.1"}
		if t.ALPN != "" && t.ALPN != "none" {
			alpn = strings.Split(t.ALPN, ",")
		}

		fingerprint = "chrome"
		if t.TlsFingerprint != "" && t.TlsFingerprint != "none" {
			fingerprint = t.TlsFingerprint
		}

		if t.AllowInsecure != "" {
			if t.AllowInsecure == "1" || t.AllowInsecure == "true" {
				insecure = true
			}
		}
	}

	var transport = &option.V2RayTransportOptions{
		Type: t.Type,
	}

	switch t.Type {
	case "tcp":
		break
	case "ws":
		transport.WebsocketOptions = option.V2RayWebsocketOptions{
			Path:                t.Path,
			Headers:             option.HTTPHeader{},
			MaxEarlyData:        0,
			EarlyDataHeaderName: "",
		}
		transport.WebsocketOptions.Headers["host"] = option.Listable[string]{t.Host}
		transport.WebsocketOptions.Headers["User-Agent"] = option.Listable[string]{"Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Safari/537.36"}

		break
	case "http":
		transport.HTTPOptions = option.V2RayHTTPOptions{
			Host:        nil,
			Path:        t.Path,
			Method:      "GET",
			Headers:     nil,
			IdleTimeout: 0,
			PingTimeout: 0,
		}
		if t.Host != "" {
			h := conf.StringList(strings.Split(t.Host, ","))
			transport.HTTPOptions.Host = option.Listable[string](h)
		}
		break
	case "httpupgrade":
		transport.HTTPUpgradeOptions = option.V2RayHTTPUpgradeOptions{
			Host:    t.Host,
			Path:    t.Path,
			Headers: nil,
		}
		break
	case "grpc":
		// t.Mode Gun & Multi
		if len(t.ServiceName) > 0 {
			if t.ServiceName[0] == '/' {
				t.ServiceName = t.ServiceName[1:]
			}
		}
		transport.GRPCOptions = option.V2RayGRPCOptions{
			ServiceName: t.ServiceName,
		}
		break
	case "quic":
		transport.QUICOptions = option.V2RayQUICOptions{}
		break
	}

	opts := option.TrojanOutboundOptions{
		DialerOptions: option.DialerOptions{},
		ServerOptions: option.ServerOptions{
			Server:     t.Address,
			ServerPort: uint16(port),
		},
		Password:  t.Password,
		Transport: transport,
		OutboundTLSOptionsContainer: option.OutboundTLSOptionsContainer{
			TLS: &option.OutboundTLSOptions{
				Enabled:    tls,
				ServerName: t.SNI,
				ALPN:       alpn,
				UTLS: &option.OutboundUTLSOptions{
					Enabled:     true,
					Fingerprint: fingerprint,
				},
				Insecure: insecure,
			},
		},
	}
	if t.Security == "reality" {
		opts.TLS.Reality = &option.OutboundRealityOptions{
			Enabled:   true,
			PublicKey: t.PublicKey,
			ShortID:   t.ShortIds,
		}
	}

	return &option.Outbound{
		Type:          t.Name(),
		TrojanOptions: opts,
	}, nil
}

func (t *Trojan) CraftOutbound(ctx context.Context, l logger.ContextLogger, allowInsecure bool) (adapter.Outbound, error) {

	options, err := t.CraftOutboundOptions(allowInsecure)
	if err != nil {
		return nil, err
	}

	out, err := outbound.New(ctx, adapter.RouterFromContext(ctx), l, "out_trojan", *options)
	if err != nil {
		return nil, errors.New(fmt.Sprintf("failed creating trojan outbound: %v", err))
	}

	return out, nil
}
