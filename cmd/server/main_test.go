package main

import "testing"

func TestServerAddress(t *testing.T) {
	tests := []struct {
		name    string
		listen  string
		port    string
		expects string
	}{
		{name: "all interfaces", port: "8080", expects: ":8080"},
		{name: "IPv4", listen: "127.0.0.1", port: "9000", expects: "127.0.0.1:9000"},
		{name: "IPv6", listen: "::1", port: "9000", expects: "[::1]:9000"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := serverAddress(tt.listen, tt.port); got != tt.expects {
				t.Fatalf("serverAddress() = %q, want %q", got, tt.expects)
			}
		})
	}
}
