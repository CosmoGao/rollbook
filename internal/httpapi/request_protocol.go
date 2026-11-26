package httpapi

import (
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

// publicOrigin 使用直接连接信息；仅可信直接代理可提供单值转发地址。
// 代理应覆盖客户端传入的转发头，避免客户端伪造公开入口。
func (e Engine) publicOrigin(r *http.Request) (string, bool) {
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	host := r.Host
	if e.trustedPeer(r) {
		xProto, xp, xv := singleProtocol(r.Header.Values("X-Forwarded-Proto"))
		fProto, fp, fv := forwardedProtocol(r.Header.Values("Forwarded"))
		if !xv || !fv || (xp && fp && xProto != fProto) {
			return "", false
		}
		if xp {
			scheme = xProto
		} else if fp {
			scheme = fProto
		}
		xHost, xhp, xhv := singleForwardedValue(r.Header.Values("X-Forwarded-Host"))
		fHost, fhp, fhv := forwardedHost(r.Header.Values("Forwarded"))
		port, pp, pv := singleForwardedValue(r.Header.Values("X-Forwarded-Port"))
		if !xhv || !fhv || !pv {
			return "", false
		}
		if pp {
			if !validPort(port) {
				return "", false
			}
		}
		if xhp && fhp && xHost != fHost {
			return "", false
		}
		if xhp {
			host = xHost
		} else if fhp {
			host = fHost
		}
		if pp {
			if existing := portOfHost(host); existing != "" {
				if existing != port {
					return "", false
				}
			} else {
				host += ":" + port
			}
		}
	}
	if !validPublicHost(host) {
		return "", false
	}
	return scheme + "://" + host, true
}

func (e Engine) trustedPeer(r *http.Request) bool {
	peerHost, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	peer, err := netip.ParseAddr(peerHost)
	if err != nil {
		return false
	}
	for _, prefix := range e.TrustedProxyCIDRs {
		if prefix.Contains(peer.Unmap()) || prefix.Contains(peer) {
			return true
		}
	}
	return false
}

func singleForwardedValue(values []string) (string, bool, bool) {
	if len(values) == 0 {
		return "", false, true
	}
	if len(values) != 1 || values[0] == "" || strings.TrimSpace(values[0]) != values[0] || strings.ContainsAny(values[0], ", \t\r\n\"\\") {
		return "", true, false
	}
	return values[0], true, true
}

func forwardedHost(values []string) (string, bool, bool) {
	if len(values) == 0 {
		return "", false, true
	}
	if len(values) != 1 || strings.Contains(values[0], ",") {
		return "", true, false
	}
	var host string
	for _, item := range strings.Split(values[0], ";") {
		pair := strings.SplitN(strings.TrimSpace(item), "=", 2)
		if len(pair) != 2 {
			return "", true, false
		}
		if strings.EqualFold(strings.TrimSpace(pair[0]), "host") {
			if host != "" {
				return "", true, false
			}
			var ok bool
			host, _, ok = singleForwardedValue([]string{pair[1]})
			if !ok {
				return "", true, false
			}
		}
	}
	return host, host != "", true
}

func validPort(raw string) bool {
	n, err := strconv.Atoi(raw)
	return err == nil && n > 0 && n <= 65535 && strconv.Itoa(n) == raw
}
func portOfHost(host string) string {
	if strings.HasPrefix(host, "[") {
		_, port, err := net.SplitHostPort(host)
		if err == nil {
			return port
		}
		return ""
	}
	if i := strings.LastIndex(host, ":"); i >= 0 {
		return host[i+1:]
	}
	return ""
}
func validPublicHost(host string) bool {
	u, err := url.Parse("https://" + host)
	if err != nil || u.Host != host || u.Hostname() == "" || u.User != nil || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || strings.ContainsAny(host, "@/#?% \\ \t\r\n") {
		return false
	}
	port := portOfHost(host)
	return port == "" || validPort(port)
}

func (e Engine) sameOrigin(r *http.Request) bool {
	values := r.Header.Values("Origin")
	if len(values) == 0 {
		return true
	}
	if len(values) != 1 {
		return false
	}
	origin, ok := e.publicOrigin(r)
	return ok && values[0] == origin
}

// secureRequest 仅采信直接 TCP 对端为可信代理时传入的协议头。
func (e Engine) secureRequest(r *http.Request) bool {
	origin, valid := e.publicOrigin(r)
	return valid && strings.HasPrefix(origin, "https://")
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
