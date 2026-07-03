package findings

import (
	"testing"
	"time"
)

func TestReportExitCodeContract(t *testing.T) {
	ref := LiveResource("Node", ScopeCluster, "", "node-a", "uid-node-a")
	for _, tc := range []struct {
		name string
		fs   []Finding
		want int
	}{
		{name: "clean", want: 0},
		{name: "warnings only", fs: []Finding{{Severity: SeverityWarning, Resources: []ResourceReference{ref}}}, want: 1},
		{name: "blocker", fs: []Finding{{Severity: SeverityBlocker, Resources: []ResourceReference{ref}}}, want: 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rpt := NewReport("1.36", "test", "", time.Time{}, tc.fs)
			if got := rpt.ExitCode(); got != tc.want {
				t.Fatalf("ExitCode() = %d, want %d", got, tc.want)
			}
		})
	}
}
