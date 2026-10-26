package generator

import "testing"

func TestProtocolToolGoMinimum(t *testing.T) {
	for _, value := range []string{"go1.26.4", "go1.26.5", "go1.27.2"} {
		if !compatibleProtocolGo(value) {
			t.Errorf("compatible plugin compiler rejected: %s", value)
		}
	}
	for _, value := range []string{"", "unknown", "devel go1.28", "go1.26.3", "go1.26rc1", "go1.25.9"} {
		if compatibleProtocolGo(value) {
			t.Errorf("unsupported plugin compiler accepted: %s", value)
		}
	}
}
