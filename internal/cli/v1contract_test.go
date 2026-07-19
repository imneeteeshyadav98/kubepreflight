package cli

import (
	"testing"

	"github.com/imneeteeshyadav98/kubepreflight/internal/v1compat"
)

func TestV1CompatibilityContractCurrentImplementation(t *testing.T) {
	report := v1compat.Check(V1CompatibilityActual())
	if !report.OK() {
		t.Fatalf("v1 compatibility contract failed:\n%s", report.String())
	}
}
