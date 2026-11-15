package server

import (
	"context"
	"errors"
	"net"
	"testing"
)

func TestIsInternalURL(t *testing.T) {
	tests := []struct {
		url      string
		expected bool
	}{
		{"http://192.168.1.1:8080", true},
		{"http://10.0.0.5", true},
		{"http://172.16.0.1", true},
		{"http://172.31.255.255", true},
		{"http://172.32.0.1", false}, // outside private range
		{"http://127.0.0.1", true},
		{"http://127.0.0.1:7000", true},
		{"http://127.0.0.1:7000/secret", true},
		{"http://169.254.0.5", true},
		{"http://localhost", true}, // resolves to 127.0.0.1
		{"http://0.0.0.0:80", true},
		{"http://8.8.8.8", false},
		{"http://google.com", false}, // resolves to public IPs
		{"http://[::1]/", true},
		{"http://[::1]:7000/", true},
		{"invalid-url", false}, // invalid format
		{"", false},            // empty string
	}
	for _, test := range tests {
		result := IsInternalURL(test.url)
		if result != test.expected {
			t.Errorf("IsInternalURL(%q) = %v; want %v", test.url, result, test.expected)
		}
	}
}

func TestIsInternalURLResolvesHostNames(t *testing.T) {
	defer func(lookup func(context.Context, string) ([]net.IPAddr, error)) { lookupIPAddr = lookup }(lookupIPAddr)
	lookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		switch host {
		case "internal.example":
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}, {IP: net.ParseIP("10.0.0.1")}}, nil
		case "public.example":
			return []net.IPAddr{{IP: net.ParseIP("93.184.216.34")}}, nil
		}
		return nil, errors.New("no such host")
	}
	for url, want := range map[string]bool{
		"http://internal.example/a": true,
		"http://public.example/a":   false,
		"http://unknown.example/a":  false,
	} {
		if have := IsInternalURL(url); have != want {
			t.Errorf("IsInternalURL(%q) = %v; want %v", url, have, want)
		}
	}
}
