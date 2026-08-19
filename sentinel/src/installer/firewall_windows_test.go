package installer

import (
	"fmt"
	"net"
	"testing"
)

func TestCheckPortAvailable(t *testing.T) {
	// Find a dynamically free port
	ln, err := net.Listen("tcp", "0.0.0.0:0")
	if err != nil {
		t.Fatalf("failed to listen on dynamic port: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	// Port is currently occupied by ln
	if err := CheckPortAvailable(port); err == nil {
		t.Errorf("expected error for occupied port %d, got nil", port)
	}

	// Close listener
	ln.Close()

	// Now port should be available
	if err := CheckPortAvailable(port); err != nil {
		t.Errorf("expected port %d to be available, got: %v", port, err)
	}
}

func TestParsePort(t *testing.T) {
	tests := []struct {
		input   string
		want    int
		wantErr bool
	}{
		{"9876", 9876, false},
		{"80", 80, false},
		{"65535", 65535, false},
		{"0", 0, true},
		{"-1", 0, true},
		{"65536", 0, true},
		{"abc", 0, true},
		{"", 0, true},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("port_%s", tt.input), func(t *testing.T) {
			got, err := ParsePort(tt.input)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParsePort(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Fatalf("ParsePort(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}
