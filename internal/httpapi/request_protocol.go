package httpapi

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// secureRequest 仅采信直接 TCP 对端为可信代理时传入的协议头。
func (e Engine) secureRequest(r *http.Request) bool {
	if r.TLS != nil {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return false
	}
	trusted := false
	for _, prefix := range e.TrustedProxyCIDRs {
		if prefix.Contains(peer.Unmap()) || prefix.Contains(peer) {
			trusted = true
			break
		}
	}
	if !trusted {
		return false
	}
	xProto, xPresent, xValid := singleProtocol(r.Header.Values("X-Forwarded-Proto"))
	fProto, fPresent, fValid := forwardedProtocol(r.Header.Values("Forwarded"))
	if !xValid || !fValid || (xPresent && fPresent && xProto != fProto) {
		return false
	}
	return xPresent && xProto == "https" || fPresent && fProto == "https"
}

func singleProtocol(values []string) (string, bool, bool) {
	if len(values) == 0 {
		return "", false, true
	}
	if len(values) != 1 {
		return "", true, false
	}
	value := strings.ToLower(strings.TrimSpace(values[0]))
	return value, true, value == "http" || value == "https"
}

func forwardedProtocol(values []string) (string, bool, bool) {
	if len(values) == 0 {
		return "", false, true
	}
	if len(values) != 1 || strings.Contains(values[0], ",") {
		return "", true, false
	}
	var proto string
	for _, field := range strings.Split(values[0], ";") {
		pair := strings.SplitN(strings.TrimSpace(field), "=", 2)
		if len(pair) != 2 || strings.TrimSpace(pair[0]) == "" {
			return "", true, false
		}
		if !strings.EqualFold(strings.TrimSpace(pair[0]), "proto") {
			continue
		}
		if proto != "" {
			return "", true, false
		}
		proto = strings.ToLower(strings.TrimSpace(pair[1]))
		if proto != "http" && proto != "https" {
			return "", true, false
		}
	}
	return proto, true, proto != ""
}
