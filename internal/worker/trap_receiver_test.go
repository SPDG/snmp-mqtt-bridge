package worker

import (
	"errors"
	"net"
	"testing"

	"snmp-mqtt-bridge/internal/domain"
)

func TestFindDeviceIDByTrapSource(t *testing.T) {
	devices := []domain.Device{
		{ID: "ups-1", IPAddress: "ups.local"},
		{ID: "pdu-1", IPAddress: "192.168.1.50"},
		{ID: "pdu-2", IPAddress: "pdu-rack1.lan"},
		{ID: "ups-v6", IPAddress: "ups-v6.local"},
	}

	lookup := func(host string) ([]string, error) {
		switch host {
		case "ups.local":
			return []string{"192.168.1.10"}, nil
		case "pdu-rack1.lan":
			return []string{"192.168.1.100", "10.0.0.10"}, nil
		case "ups-v6.local":
			return []string{"2001:db8::10"}, nil
		default:
			return nil, errors.New("not found")
		}
	}

	tests := []struct {
		name     string
		sourceIP string
		wantID   string
	}{
		{name: "matches literal IPv4", sourceIP: "192.168.1.50", wantID: "pdu-1"},
		{name: "matches resolved hostname", sourceIP: "192.168.1.100", wantID: "pdu-2"},
		{name: "matches resolved IPv6 hostname", sourceIP: "2001:db8::10", wantID: "ups-v6"},
		{name: "returns nil for unknown source", sourceIP: "192.168.1.200"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := findDeviceIDByTrapSource(devices, net.ParseIP(tt.sourceIP), lookup)
			if tt.wantID == "" {
				if got != nil {
					t.Fatalf("findDeviceIDByTrapSource() = %q, want nil", *got)
				}
				return
			}
			if got == nil || *got != tt.wantID {
				var gotID string
				if got != nil {
					gotID = *got
				}
				t.Fatalf("findDeviceIDByTrapSource() = %q, want %q", gotID, tt.wantID)
			}
		})
	}
}

func TestMatchDeviceAddressDoesNotResolveLiteralIP(t *testing.T) {
	called := false
	lookup := func(string) ([]string, error) {
		called = true
		return nil, errors.New("unexpected lookup")
	}

	if !matchDeviceAddress("192.168.1.50", "192.168.1.50", lookup) {
		t.Fatal("matchDeviceAddress() = false, want true")
	}
	if called {
		t.Fatal("matchDeviceAddress resolved a literal IP address")
	}
}

func TestMatchDeviceAddressReturnsFalseOnLookupError(t *testing.T) {
	lookup := func(string) ([]string, error) {
		return nil, errors.New("not found")
	}

	if matchDeviceAddress("missing.local", "192.168.1.50", lookup) {
		t.Fatal("matchDeviceAddress() = true, want false")
	}
}
