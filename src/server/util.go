package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"slices"
	"time"

	"github.com/nkanaev/yarr/src/client"
)

func newCrawlClient() *http.Client {
	httpClient := client.NewBuilder().Build()
	httpClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		if IsInternalURL(req.URL.String()) {
			return fmt.Errorf("redirect to internal address %s", req.URL.Host)
		}
		return nil
	}
	return httpClient
}

// IsInternalURL reports whether a URL points at this machine or the local network,
// also by a host name resolving to such an address.
func IsInternalURL(urlStr string) bool {
	parsedURL, err := url.Parse(urlStr)
	if err != nil {
		return false
	}

	host := parsedURL.Hostname()
	if host == "" {
		return false
	}
	if host == "localhost" {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return isInternalIP(ip)
	}

	// a host name may point at an internal address just as well. one that cannot
	// be resolved cannot be fetched either
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	addrs, err := lookupIPAddr(ctx, host)
	if err != nil {
		return false
	}
	return slices.ContainsFunc(addrs, func(addr net.IPAddr) bool { return isInternalIP(addr.IP) })
}

var lookupIPAddr = net.DefaultResolver.LookupIPAddr

func isInternalIP(ip net.IP) bool {
	return ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified()
}
