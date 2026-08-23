// Package config config_normalize.go - 配置归一化
package config

import (
	"fmt"
	"net"
	"net/url"
	"strings"
)

func NormalizeAllowedOrigins(values []string) ([]string, error) {
	seen := make(map[string]struct{})
	origins := make([]string, 0, len(values))
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			origin := strings.TrimSpace(item)
			if origin == "" {
				continue
			}
			parsed, err := url.Parse(origin)
			if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" ||
				parsed.User != nil || (parsed.Path != "" && parsed.Path != "/") || parsed.RawQuery != "" || parsed.Fragment != "" {
				return nil, fmt.Errorf("invalid allowed origin %q: expected http(s)://host[:port]", origin)
			}
			normalized := strings.ToLower(parsed.Scheme + "://" + parsed.Host)
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			origins = append(origins, normalized)
		}
	}
	return origins, nil
}
func NormalizeTrustedProxies(values []string) ([]string, error) {
	seen := make(map[string]struct{})
	proxies := make([]string, 0, len(values))
	for _, value := range values {
		for _, item := range strings.Split(value, ",") {
			candidate := strings.TrimSpace(item)
			if candidate == "" {
				continue
			}

			normalized := ""
			if ip := net.ParseIP(candidate); ip != nil {
				normalized = ip.String()
			} else if _, network, err := net.ParseCIDR(candidate); err == nil {
				normalized = network.String()
			} else {
				return nil, fmt.Errorf("invalid trusted proxy %q: expected an IP address or CIDR", candidate)
			}
			if _, ok := seen[normalized]; ok {
				continue
			}
			seen[normalized] = struct{}{}
			proxies = append(proxies, normalized)
		}
	}
	return proxies, nil
}
