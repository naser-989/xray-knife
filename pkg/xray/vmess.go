package xray

import (
	"encoding/json"
	"fmt"
	"github.com/naser-989/xray-knife/v3/pkg/protocol"
	"net"
	"net/url"
	"strings"

	"github.com/fatih/color"
	"github.com/naser-989/xray-knife/v3/utils"
	"github.com/xtls/xray-core/infra/conf"
)

func NewVmess(link string) Protocol {
	return &Vmess{OrigLink: link}
}

func (v *Vmess) Name() string {
	return "vmess"
}

func method1(v *Vmess, link string) error {
	b64encoded := link[8:]
	decoded, err := utils.Base64Decode(b64encoded)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(decoded, v); err != nil {
		return err
	}

	if utils.IsIPv6(v.Address) {
		v.Address = "[" + v.Address + "]"
	}
	return nil
}

// Example:
// vmess://YXV0bzpjYmI0OTM1OC00NGQxLTQ4MmYtYWExNC02ODA3NzNlNWNjMzdAc25hcHBmb29kLmlyOjQ0Mw?remarks=sth&obfsParam=huhierg.com&path=/&obfs=websocket&tls=1&peer=gdfgreg.com&alterId=0
func method2(v *Vmess, link string) error {
	uri, err := url.Parse(link)
	if err != nil {
		return err
	}
	decoded, err := utils.Base64Decode(uri.Host)
	if err != nil {
		return err
	}
	link = protocol.VmessIdentifier + "://" + string(decoded) + "?" + uri.RawQuery

	uri, err = url.Parse(link)
	if err != nil {
		return err
	}

	v.Security = uri.User.Username()
	v.ID, _ = uri.User.Password()

	v.Address, v.Port, err = net.SplitHostPort(uri.Host)
	if err != nil {
		return err
	}

	if utils.IsIPv6(v.Address) {
		v.Address = "[" + v.Address + "]"
	}
	//parseUint, err := strconv.ParseUint(suhp[2], 10, 16)
	//if err != nil {
	//	return err
	//}

	v.Aid = "0"

	queryValues := uri.Query()
	if value := queryValues.Get("remarks"); value != "" {
		v.Remark = value
	}

	if value := queryValues.Get("path"); value != "" {
		v.Path = value
	}

	if value := queryValues.Get("tls"); value == "1" {
		v.TLS = "tls"
	}

	if value := queryValues.Get("obfs"); value != "" {
		switch value {
		case "websocket":
			v.Network = "ws"
			v.Type = "none"
		case "none":
			v.Network = "tcp"
			v.Type = "none"
		}
	}
	if value := queryValues.Get("obfsParam"); value != "" {
		v.Host = value
	}
	if value := queryValues.Get("peer"); value != "" {
		v.SNI = value
	} else {
		if v.TLS == "tls" {
			v.SNI = v.Host
		}
	}

	return nil
}

//func method3(v *Vmess, link string) error {
//
//}

func (v *Vmess) Parse() error {
	if !strings.HasPrefix(v.OrigLink, protocol.VmessIdentifier) {
		return fmt.Errorf("vmess unreconized: %s", v.OrigLink)
	}

	var err error = nil

	if err = method1(v, v.OrigLink); err != nil {
		if err = method2(v, v.OrigLink); err != nil {
			return err
		}
	}

	if v.Type == "xhttp" || v.Type == "http" || v.Network == "ws" || v.Network == "h2" {
		if v.Path == "" {
			v.Path = "/"
		}
	}

	return err
}

func (v *Vmess) DetailsStr() string {
	copyV := *v
	info := fmt.Sprintf("%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %v\n%s: %s\n",
		color.RedString("Protocol"), v.Name(),
		color.RedString("Remark"), copyV.Remark,
		color.RedString("Network"), copyV.Network,
		color.RedString("Address"), copyV.Address,
		color.RedString("Port"), copyV.Port,
		color.RedString("UUID"), copyV.ID)

	if copyV.Network == "" {

	} else if copyV.Type == "xhttp" || copyV.Type == "http" || copyV.Network == "httpupgrade" || copyV.Network == "ws" || copyV.Network == "h2" {
		if copyV.Type == "" {
			copyV.Type = "none"
		}
		if copyV.Host == "" {
			copyV.Host = "none"
		}
		if copyV.Path == "" {
			copyV.Path = "none"
		}

		info += fmt.Sprintf("%s: %s\n%s: %s\n%s: %s\n",
			color.RedString("Type"), copyV.Type,
			color.RedString("Host"), copyV.Host,
			color.RedString("Path"), copyV.Path)
	} else if copyV.Network == "kcp" {
		info += fmt.Sprintf("%s: %s\n", color.RedString("KCP Seed"), copyV.Path)
	} else if copyV.Network == "grpc" {
		if copyV.Host == "" {
			copyV.Host = "none"
		}
		info += fmt.Sprintf("%s: %s\n%s: %s\n",
			color.RedString("ServiceName"), copyV.Path,
			color.RedString("Authority"), copyV.Host)
	}

	if len(copyV.TLS) != 0 && copyV.TLS != "none" {
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
		if len(copyV.TlsFingerprint) == 0 {
			copyV.TlsFingerprint = "none"
		}
		info += fmt.Sprintf("%s: tls\n%s: %s\n%s: %s\n%s: %s\n",
			color.RedString("TLS"),
			color.RedString("SNI"), copyV.SNI,
			color.RedString("ALPN"), copyV.ALPN,
			color.RedString("Fingerprint"), copyV.TlsFingerprint)

		if v.AllowInsecure != "" {
			info += fmt.Sprintf("%s: %v\n",
				color.RedString("Insecure"), v.AllowInsecure)
		}
	}
	return info
}

func (v *Vmess) ConvertToGeneralConfig() (g protocol.GeneralConfig) {
	g.Protocol = v.Name()
	g.Address = v.Address
	g.Aid = fmt.Sprintf("%v", v.Aid)
	g.Host = v.Host
	g.ID = v.ID
	g.Network = v.Network
	g.Path = v.Path
	g.Port = fmt.Sprintf("%v", v.Port)
	g.Remark = v.Remark
	if v.TLS == "" {
		g.TLS = "none"
	} else {
		g.TLS = v.TLS
	}
	g.TLS = v.TLS
	g.SNI = v.SNI
	g.ALPN = v.ALPN
	g.TlsFingerprint = v.TlsFingerprint
	g.Type = v.Type
	g.OrigLink = v.OrigLink

	return g
}

func (v *Vmess) BuildOutboundDetourConfig(allowInsecure bool) (*conf.OutboundDetourConfig, error) {
	out := &conf.OutboundDetourConfig{}
	out.Tag = "proxy"
	out.Protocol = v.Name()

	p := conf.TransportProtocol(v.Network)
	s := &conf.StreamConfig{
		Network:  &p,
		Security: v.TLS,
	}

	switch v.Network {
	case "tcp":
		s.TCPSettings = &conf.TCPConfig{}
		if v.Type == "" || v.Type == "none" {
			s.TCPSettings.HeaderConfig = json.RawMessage([]byte(`{ "type": "none" }`))
		} else {
			pathb, _ := json.Marshal(strings.Split(v.Path, ","))
			hostb, _ := json.Marshal(strings.Split(v.Host, ","))
			s.TCPSettings.HeaderConfig = json.RawMessage([]byte(fmt.Sprintf(`
			{
				"type": "http",
				"request": {
					"path": %s,
					"headers": {
						"Host": %s,
						"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Safari/537.36"
					}
				}
			}
			`, string(pathb), string(hostb))))
		}
		break
	case "kcp":
		s.KCPSettings = &conf.KCPConfig{}
		s.KCPSettings.HeaderConfig = json.RawMessage([]byte(fmt.Sprintf(`{ "type": "%s" }`, v.Type)))
		break
	case "ws":
		s.WSSettings = &conf.WebSocketConfig{}
		s.WSSettings.Path = v.Path
		s.WSSettings.Headers = map[string]string{
			"Host":       v.Host,
			"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/92.0.4515.131 Safari/537.36",
		}
		break
		//case "h2", "http":
	case "xhttp":
		s.XHTTPSettings = &conf.SplitHTTPConfig{
			Host: v.Host,
			Path: v.Path,
			Mode: v.Type,
		}
		//if v.Host != "" {
		//	h := conf.StringList(strings.Split(v.Host, ","))
		//	s.XHTTPSettings.Host = &h
		//}
		if v.Type == "" {
			s.XHTTPSettings.Mode = "auto"
		}
		break
	case "httpupgrade":
		s.HTTPUPGRADESettings = &conf.HttpUpgradeConfig{
			Host: v.Host,
			Path: v.Path,
		}
		break
	case "splithttp":
		s.SplitHTTPSettings = &conf.SplitHTTPConfig{
			Host: v.Host,
			Path: v.Path,
		}
		break
	case "grpc":
		if len(v.Path) > 0 {
			if v.Path[0] == '/' {
				v.Path = v.Path[1:]
			}
		}
		multiMode := false
		if v.Type != "gun" {
			multiMode = true
		}
		s.GRPCSettings = &conf.GRPCConfig{
			InitialWindowsSize: 65536,
			HealthCheckTimeout: 20,
			MultiMode:          multiMode,
			IdleTimeout:        60,
			Authority:          v.Host,
			ServiceName:        v.Path,
		}
		break
		//case "quic":
		//	t := "none"
		//	if v.Type != "" {
		//		t = v.Type
		//	}
		//	s.QUICSettings = &conf.QUICConfig{
		//		Header:   json.RawMessage([]byte(fmt.Sprintf(`{ "type": "%s" }`, t))),
		//		Security: v.Host,
		//		Key:      v.Path,
		//	}
		//	break
	}

	if v.TLS == "tls" {
		if v.TlsFingerprint == "" {
			v.TlsFingerprint = "chrome"
		}
		s.TLSSettings = &conf.TLSConfig{
			Fingerprint: v.TlsFingerprint,
			Insecure:    allowInsecure,
		}
		if v.SNI != "" {
			s.TLSSettings.ServerName = v.SNI
		} else {
			s.TLSSettings.ServerName = v.Host
		}
		if v.ALPN != "" {
			s.TLSSettings.ALPN = &conf.StringList{v.ALPN}
		}
	}

	out.StreamSetting = s
	oset := json.RawMessage(fmt.Sprintf(`{
  "vnext": [
    {
      "address": "%s",
      "port": %v,
      "users": [
        {
          "id": "%s",
          "alterId": %v,
          "security": "%s"
        }
      ]
    }
  ]
}`, v.Address, v.Port, v.ID, v.Aid, v.Security))
	out.Settings = &oset
	return out, nil
}

func (v *Vmess) BuildInboundDetourConfig() (*conf.InboundDetourConfig, error) {
	return nil, nil
}
