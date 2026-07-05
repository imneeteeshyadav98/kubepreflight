package gke

import "testing"

func TestGKE_Name(t *testing.T) {
	if got := (GKE{}).Name(); got != "gke" {
		t.Errorf("Name() = %q, want %q", got, "gke")
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
			cfg:  Config{ClusterName: "prod-gke", Project: "my-project", Location: "us-central1"},
		},
		{
			name:    "missing cluster name",
			cfg:     Config{Project: "my-project", Location: "us-central1"},
			wantErr: true,
		},
		{
			name:    "missing project",
			cfg:     Config{ClusterName: "prod-gke", Location: "us-central1"},
			wantErr: true,
		},
		{
			name:    "missing location",
			cfg:     Config{ClusterName: "prod-gke", Project: "my-project"},
			wantErr: true,
		},
		{
			name:    "missing all",
			cfg:     Config{},
			wantErr: true,
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
