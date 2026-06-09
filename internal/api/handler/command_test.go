package handler

import "testing"

func TestValidateAtenOutletName(t *testing.T) {
	tests := []struct {
		name    string
		wantErr bool
	}{
		{name: "hive 07"},
		{name: "hive_07"},
		{name: "UPPER lower 123"},
		{name: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"},
		{name: "", wantErr: true},
		{name: "hive-07", wantErr: true},
		{name: "hive.07", wantErr: true},
		{name: "zażółć", wantErr: true},
		{name: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", wantErr: true},
	}

	for _, tt := range tests {
		err := validateAtenOutletName(tt.name)
		if (err != nil) != tt.wantErr {
			t.Fatalf("validateAtenOutletName(%q) error = %v, wantErr %v", tt.name, err, tt.wantErr)
		}
	}
}
