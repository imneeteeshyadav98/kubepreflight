package aks

import "testing"

func TestAKS_Name(t *testing.T) {
	if got := (AKS{}).Name(); got != "aks" {
		t.Errorf("Name() = %q, want %q", got, "aks")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		cfg     Config
		wantErr bool
	}{
		{
			name: "all required fields present",
			cfg:  Config{ClusterName: "prod-aks", ResourceGroup: "rg-prod"},
		},
		{
			name:    "missing cluster name",
			cfg:     Config{ResourceGroup: "rg-prod"},
			wantErr: true,
		},
		{
			name:    "missing resource group",
			cfg:     Config{ClusterName: "prod-aks"},
			wantErr: true,
		},
		{
			name:    "missing both",
			cfg:     Config{},
			wantErr: true,
		},
		{
			name: "optional subscription ID omitted is fine",
			cfg:  Config{ClusterName: "prod-aks", ResourceGroup: "rg-prod", SubscriptionID: ""},
		},
		{
			name: "subscription ID present is fine too",
			cfg:  Config{ClusterName: "prod-aks", ResourceGroup: "rg-prod", SubscriptionID: "sub-123"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if tc.wantErr && err == nil {
				t.Errorf("Validate() = nil, want error for %+v", tc.cfg)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("Validate() = %v, want nil for %+v", err, tc.cfg)
			}
		})
	}
}
