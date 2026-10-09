package service

import "testing"

func TestCalculateDerivedValuesUsesPDUVoltageForOutletPower(t *testing.T) {
	poller := &PollerService{}
	values := map[string]interface{}{
		"Voltage":          230.0,
		"Total Current":    0.2,
		"Outlet 1 Current": 0.2,
		"Outlet 2 Current": 0.0,
		"Outlet 1 Voltage": 0.0,
	}

	poller.calculateDerivedValues(values)

	if got := values["Active Power"]; got != 46.0 {
		t.Fatalf("Active Power = %v, want 46", got)
	}
	if got := values["Outlet 1 Power"]; got != 46.0 {
		t.Fatalf("Outlet 1 Power = %v, want 46", got)
	}
	if got := values["Outlet 2 Power"]; got != 0.0 {
		t.Fatalf("Outlet 2 Power = %v, want 0", got)
	}
}

func TestCalculateDerivedValuesKeepsMeasuredOutletPower(t *testing.T) {
	poller := &PollerService{}
	values := map[string]interface{}{
		"Voltage":          230.0,
		"Outlet 1 Current": 0.2,
		"Outlet 1 Voltage": 229.0,
		"Outlet 1 Power":   12.5,
	}

	poller.calculateDerivedValues(values)

	if got := values["Outlet 1 Power"]; got != 12.5 {
		t.Fatalf("Outlet 1 Power = %v, want measured 12.5", got)
	}
}

func TestCalculateDerivedValuesPrefersOutletVoltage(t *testing.T) {
	poller := &PollerService{}
	values := map[string]interface{}{
		"Voltage":          230.0,
		"Outlet 3 Current": 1.0,
		"Outlet 3 Voltage": 120.0,
	}

	poller.calculateDerivedValues(values)

	if got := values["Outlet 3 Power"]; got != 120.0 {
		t.Fatalf("Outlet 3 Power = %v, want 120", got)
	}
}
