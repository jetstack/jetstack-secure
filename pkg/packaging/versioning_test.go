package packaging

import "testing"

func TestSupportsPreflightPrefix(t *testing.T) {
	testCases := []struct {
		version    string
		wantResult bool
		wantErr    bool
	}{
		{"", true, false},
		{"0.0.1", true, false},
		{"0.1.0", true, false},
		{"0.1.1", false, false},
		{"1.0.0", false, false},
		{"not-semver", false, true},
	}

	for idx, tc := range testCases {
		t.Run(string(idx), func(t *testing.T) {
			m := &PolicyManifest{SchemaVersion: tc.version}
			gotResult, err := m.SupportsPreflightPrefix()

			if err != nil && !tc.wantErr {
				t.Fatalf("expected error to be nil but got: %+v", err)
			} else if err == nil && tc.wantErr {
				t.Fatalf("expected to get an error but didn't get one")
			}

			if gotResult != tc.wantResult {
				t.Errorf("expected %s to have compatibility=%v but got compatibility=%v", tc.version, tc.wantResult, gotResult)
			}
		})
	}
}
