package protocol

import "testing"

func TestIsCompatibleVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		want    bool
	}{
		{name: "equal version is compatible", version: "1.0.0", want: true},
		{name: "lower minor same major is compatible", version: "1.0.0", want: true},
		{name: "lower patch same major is compatible", version: "1.0.0", want: true},
		{name: "higher patch same major is incompatible", version: "1.0.1", want: false},
		{name: "higher minor same major is incompatible", version: "1.1.0", want: false},
		{name: "lower minor same major compatible", version: "1.0.0", want: true},
		{name: "different major lower incompatible", version: "0.9.0", want: false},
		{name: "different major higher incompatible", version: "2.0.0", want: false},
		{name: "invalid format incompatible", version: "1.0", want: false},
		{name: "negative number incompatible", version: "1.-1.0", want: false},
		{name: "non numeric incompatible", version: "1.a.0", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := IsCompatibleVersion(tt.version)
			if got != tt.want {
				t.Fatalf("IsCompatibleVersion(%q) = %v, want %v", tt.version, got, tt.want)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name    string
		a       string
		b       string
		want    int
		wantErr bool
	}{
		{name: "equal", a: "1.0.0", b: "1.0.0", want: 0},
		{name: "a less by major", a: "1.0.0", b: "2.0.0", want: -1},
		{name: "a greater by major", a: "2.0.0", b: "1.9.9", want: 1},
		{name: "a less by minor", a: "1.1.0", b: "1.2.0", want: -1},
		{name: "a greater by minor", a: "1.2.0", b: "1.1.9", want: 1},
		{name: "a less by patch", a: "1.2.3", b: "1.2.4", want: -1},
		{name: "a greater by patch", a: "1.2.4", b: "1.2.3", want: 1},
		{name: "invalid a format", a: "1.0", b: "1.0.0", wantErr: true},
		{name: "invalid b format", a: "1.0.0", b: "1", wantErr: true},
		{name: "negative a", a: "-1.0.0", b: "1.0.0", wantErr: true},
		{name: "negative b", a: "1.0.0", b: "1.-1.0", wantErr: true},
		{name: "non numeric", a: "1.a.0", b: "1.0.0", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got, err := CompareVersions(tt.a, tt.b)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("CompareVersions(%q, %q) error = nil, want error", tt.a, tt.b)
				}
				return
			}

			if err != nil {
				t.Fatalf("CompareVersions(%q, %q) unexpected error: %v", tt.a, tt.b, err)
			}
			if got != tt.want {
				t.Fatalf("CompareVersions(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}
}

func TestParseVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		wantMajor int
		wantMinor int
		wantPatch int
		wantErr   bool
	}{
		{name: "valid zero version", version: "0.0.0", wantMajor: 0, wantMinor: 0, wantPatch: 0},
		{name: "valid normal version", version: "1.2.3", wantMajor: 1, wantMinor: 2, wantPatch: 3},
		{name: "valid multi digit", version: "10.20.30", wantMajor: 10, wantMinor: 20, wantPatch: 30},
		{name: "missing patch", version: "1.2", wantErr: true},
		{name: "missing minor and patch", version: "1", wantErr: true},
		{name: "too many parts", version: "1.2.3.4", wantErr: true},
		{name: "non numeric", version: "1.a.3", wantErr: true},
		{name: "negative major", version: "-1.0.0", wantErr: true},
		{name: "negative minor", version: "1.-1.0", wantErr: true},
		{name: "negative patch", version: "1.0.-1", wantErr: true},
		{name: "leading whitespace invalid", version: " 1.2.3", wantErr: true},
		{name: "trailing whitespace invalid", version: "1.2.3 ", wantErr: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			major, minor, patch, err := parseVersion(tt.version)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseVersion(%q) error = nil, want error", tt.version)
				}
				return
			}

			if err != nil {
				t.Fatalf("parseVersion(%q) unexpected error: %v", tt.version, err)
			}

			if major != tt.wantMajor || minor != tt.wantMinor || patch != tt.wantPatch {
				t.Fatalf(
					"parseVersion(%q) = (%d,%d,%d), want (%d,%d,%d)",
					tt.version,
					major,
					minor,
					patch,
					tt.wantMajor,
					tt.wantMinor,
					tt.wantPatch,
				)
			}
		})
	}
}
