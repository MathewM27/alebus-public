package httpapi

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPKey struct{}

type clientProtoKey struct{}

type trustedProxySet struct {
	ips     map[netip.Addr]struct{}
	prefixes []netip.Prefix
}

func newTrustedProxySet(entries []string) trustedProxySet {
	set := trustedProxySet{ips: map[netip.Addr]struct{}{}, prefixes: nil}
	for _, raw := range entries {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		if strings.Contains(raw, "/") {
			if p, err := netip.ParsePrefix(raw); err == nil {
				set.prefixes = append(set.prefixes, p)
			}
			continue
		}
		if ip, err := netip.ParseAddr(raw); err == nil {
			set.ips[ip] = struct{}{}
		}
	}
	return set
}

func (s trustedProxySet) contains(ip netip.Addr) bool {
	if _, ok := s.ips[ip]; ok {
		return true
	}
	for _, p := range s.prefixes {
		if p.Contains(ip) {
			return true
		}
	}
	return false
}

func ClientIPFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(clientIPKey{}).(string); ok {
		return v
	}
	return ""
}

func ClientProtoFromContext(ctx context.Context) string {
	if v, ok := ctx.Value(clientProtoKey{}).(string); ok {
		return v
	}
	return ""
}

// TrustedProxyMiddleware populates client IP/proto in context.
//
// It only trusts X-Forwarded-* headers when the TCP peer is in the trusted proxy list.
// If proxies is empty, it never trusts forwarded headers (but still sets client ip to RemoteAddr).
func TrustedProxyMiddleware(proxies []string) func(http.Handler) http.Handler {
	set := newTrustedProxySet(proxies)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip := peerIP(r.RemoteAddr)
			proto := "http"
			if r.TLS != nil {
				proto = "https"
			}

			if ip != "" {
				if addr, err := netip.ParseAddr(ip); err == nil && set.contains(addr) {
					if fwdFor := clientIPFromXForwardedFor(r.Header.Get("X-Forwarded-For")); fwdFor != "" {
						ip = fwdFor
					}
					if fwdProto := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")); fwdProto != "" {
						fwdProto = strings.ToLower(fwdProto)
						if fwdProto == "http" || fwdProto == "https" {
							proto = fwdProto
						}
					}
				}
			}

			ctx := context.WithValue(r.Context(), clientIPKey{}, ip)
			ctx = context.WithValue(ctx, clientProtoKey{}, proto)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func peerIP(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return ""
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		if addr, err := netip.ParseAddr(host); err == nil {
			return addr.Unmap().String()
		}
		return host
	}
	// Might already be an IP without a port.
	if addr, err := netip.ParseAddr(remoteAddr); err == nil {
		return addr.Unmap().String()
	}
	return ""
}

func clientIPFromXForwardedFor(xff string) string {
	// X-Forwarded-For: client, proxy1, proxy2
	parts := strings.Split(xff, ",")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || strings.EqualFold(p, "unknown") {
			continue
		}
		// Strip optional port.
		if host, _, err := net.SplitHostPort(p); err == nil {
			p = host
		}
		if net.ParseIP(p) != nil {
			return p
		}
	}
	return ""
}
