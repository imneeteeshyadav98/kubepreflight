package plan

import "testing"

func TestParseMajorMinor(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantMajor int
		wantMinor int
		wantErr   bool
	}{
		{name: "bare major.minor", input: "1.29", wantMajor: 1, wantMinor: 29},
		{name: "v-prefixed with patch and build suffix", input: "v1.29.6-eks-abc1234", wantMajor: 1, wantMinor: 29},
		{name: "patch, no build suffix", input: "1.29.0", wantMajor: 1, wantMinor: 29},
		{name: "malformed: no dot", input: "1", wantErr: true},
		{name: "malformed: empty", input: "", wantErr: true},
		{name: "malformed: non-numeric", input: "garbage", wantErr: true},
		{name: "malformed: non-numeric minor", input: "1.x", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			major, minor, err := ParseMajorMinor(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseMajorMinor(%q) succeeded, want error", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseMajorMinor(%q) = %v", tc.input, err)
			}
			if major != tc.wantMajor || minor != tc.wantMinor {
				t.Fatalf("ParseMajorMinor(%q) = %d.%d, want %d.%d", tc.input, major, minor, tc.wantMajor, tc.wantMinor)
			}
		})
	}
}

func TestCompareMinor(t *testing.T) {
	tests := []struct {
		name    string
		a, b    string
		want    int
		wantErr bool
	}{
		{name: "less", a: "1.29", b: "1.30", want: -1},
		{name: "equal", a: "1.29", b: "1.29.6", want: 0},
		{name: "greater", a: "1.31", b: "1.30", want: 1},
		{name: "cross-major rejected", a: "1.29", b: "2.0", wantErr: true},
		{name: "unparseable a", a: "bad", b: "1.30", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := CompareMinor(tc.a, tc.b)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("CompareMinor(%q, %q) succeeded, want error", tc.a, tc.b)
				}
				return
			}
			if err != nil {
				t.Fatalf("CompareMinor(%q, %q) = %v", tc.a, tc.b, err)
			}
			if got != tc.want {
				t.Fatalf("CompareMinor(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestGenerateHops_MultiHop(t *testing.T) {
	hops, err := GenerateHops("1.29", "1.33")
	if err != nil {
		t.Fatalf("GenerateHops: %v", err)
	}
	want := []Hop{
		{Index: 1, From: "1.29", To: "1.30"},
		{Index: 2, From: "1.30", To: "1.31"},
		{Index: 3, From: "1.31", To: "1.32"},
		{Index: 4, From: "1.32", To: "1.33"},
	}
	if len(hops) != len(want) {
		t.Fatalf("GenerateHops(1.29, 1.33) = %d hops, want %d: %+v", len(hops), len(want), hops)
	}
	for i, h := range hops {
		if h != want[i] {
			t.Errorf("hop %d = %+v, want %+v", i, h, want[i])
		}
	}
}

func TestGenerateHops_SingleHop(t *testing.T) {
	hops, err := GenerateHops("1.35", "1.36")
	if err != nil {
		t.Fatalf("GenerateHops: %v", err)
	}
	if len(hops) != 1 {
		t.Fatalf("GenerateHops(1.35, 1.36) = %d hops, want 1: %+v", len(hops), hops)
	}
	want := Hop{Index: 1, From: "1.35", To: "1.36"}
	if hops[0] != want {
		t.Errorf("hop 0 = %+v, want %+v", hops[0], want)
	}
}

func TestGenerateHops_RejectsSameVersion(t *testing.T) {
	if _, err := GenerateHops("1.29", "1.29"); err == nil {
		t.Fatal("GenerateHops(1.29, 1.29) succeeded, want error (nothing to plan)")
	}
}

func TestGenerateHops_RejectsDowngrade(t *testing.T) {
	if _, err := GenerateHops("1.30", "1.29"); err == nil {
		t.Fatal("GenerateHops(1.30, 1.29) succeeded, want error (downgrade)")
	}
}

func TestGenerateHops_RejectsCrossMajor(t *testing.T) {
	if _, err := GenerateHops("1.29", "2.0"); err == nil {
		t.Fatal("GenerateHops(1.29, 2.0) succeeded, want error (cross-major)")
	}
}

func TestGenerateHops_RejectsUnparseable(t *testing.T) {
	if _, err := GenerateHops("garbage", "1.30"); err == nil {
		t.Fatal("GenerateHops(garbage, 1.30) succeeded, want error")
	}
	if _, err := GenerateHops("1.29", "garbage"); err == nil {
		t.Fatal("GenerateHops(1.29, garbage) succeeded, want error")
	}
}
