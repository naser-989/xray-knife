package singbox

import (
	"errors"
	"github.com/naser-989/xray-knife/v3/pkg/protocol"
	"net/url"
	"strings"
)

func (c *Core) CreateProtocol(configLink string) (protocol.Protocol, error) {
	// Remove any spaces
	configLink = strings.TrimSpace(configLink)

	// Parse url
	uri, err := url.Parse(configLink)
	if err != nil {
		return nil, err
	}

	switch uri.Scheme {
	case protocol.VmessIdentifier:
		return NewVmess(configLink), nil
	case protocol.VlessIdentifier:
		return NewVless(configLink), nil
	case protocol.ShadowsocksIdentifier:
		return NewShadowsocks(configLink), nil
	case protocol.TrojanIdentifier:
		return NewTrojan(configLink), nil
	case protocol.SocksIdentifier:
		return NewSocks(configLink), nil
	case protocol.WireguardIdentifier:
		return NewWireguard(configLink), nil
	case protocol.Hysteria2Identifier:
		return NewHysteria2(configLink), nil
	case "hy2":
		return NewHysteria2(configLink), nil

	default:
		return nil, errors.New("invalid singbox protocol")
	}
}
