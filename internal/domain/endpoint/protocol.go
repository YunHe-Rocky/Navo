package endpoint

type Protocol string

const (
	ProtocolVLESS       Protocol = "vless"
	ProtocolVMess       Protocol = "vmess"
	ProtocolTrojan      Protocol = "trojan"
	ProtocolShadowsocks Protocol = "shadowsocks"
	ProtocolHysteria2   Protocol = "hysteria2"
	ProtocolTUIC        Protocol = "tuic"
	ProtocolWireGuard   Protocol = "wireguard"
	ProtocolHTTP        Protocol = "http"
	ProtocolHTTPS       Protocol = "https"
	ProtocolSOCKS5      Protocol = "socks5"
)

func (p Protocol) Valid() bool {
	switch p {
	case ProtocolVLESS, ProtocolVMess, ProtocolTrojan, ProtocolShadowsocks,
		ProtocolHysteria2, ProtocolTUIC, ProtocolWireGuard, ProtocolHTTP,
		ProtocolHTTPS, ProtocolSOCKS5:
		return true
	default:
		return false
	}
}
