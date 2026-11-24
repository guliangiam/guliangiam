package auth

import (
	"reflect"
	"testing"
)

func TestCasbinActionCandidates(t *testing.T) {
	tests := []struct {
		method string
		want   []string
	}{
		{"GET", []string{"get", "read"}},
		{"HEAD", []string{"head", "read"}},
		{"POST", []string{"post", "write"}},
		{"PUT", []string{"put", "write"}},
		{"PATCH", []string{"patch", "write"}},
		{"DELETE", []string{"delete"}},
		{"OPTIONS", []string{"options"}},
		{"", []string{""}},
	}
	for _, testCase := range tests {
		if got := casbinActionCandidates(testCase.method); !reflect.DeepEqual(got, testCase.want) {
			t.Errorf("casbinActionCandidates(%q) = %v, want %v", testCase.method, got, testCase.want)
		}
	}
}
