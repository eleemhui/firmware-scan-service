package main

import (
	"fmt"
	"strings"
	"testing"
)

func TestRandomCVEs_CountInRange(t *testing.T) {
	for i := range 100 {
		_ = i
		vulns := randomCVEs()
		if len(vulns) < 1 || len(vulns) > 3 {
			t.Errorf("expected 1–3 CVEs, got %d: %v", len(vulns), vulns)
		}
	}
}

func TestRandomCVEs_NoDuplicates(t *testing.T) {
	for range 100 {
		vulns := randomCVEs()
		seen := make(map[string]bool)
		for _, v := range vulns {
			if seen[v] {
				t.Errorf("duplicate CVE in result: %v", vulns)
				break
			}
			seen[v] = true
		}
	}
}

func TestRandomCVEs_ValidRange(t *testing.T) {
	for range 200 {
		for _, cve := range randomCVEs() {
			var n int
			if _, err := fmt.Sscanf(cve, "CVE-%d", &n); err != nil {
				t.Errorf("CVE %q does not match expected format CVE-NNN", cve)
				continue
			}
			if n < 1 || n > 100 {
				t.Errorf("CVE number %d out of range [1, 100]: %s", n, cve)
			}
		}
	}
}

func TestRandomCVEs_Format(t *testing.T) {
	for range 50 {
		for _, cve := range randomCVEs() {
			if !strings.HasPrefix(cve, "CVE-") {
				t.Errorf("CVE %q does not start with CVE-", cve)
			}
			if len(cve) != 7 { // "CVE-" (4) + 3 digits = 7
				t.Errorf("CVE %q has unexpected length %d, want 7", cve, len(cve))
			}
		}
	}
}
