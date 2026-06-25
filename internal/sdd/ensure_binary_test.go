package sdd

import (
	"os/exec"
	"testing"
)

// TestEnsureSddBinaryScript runs the POSIX-sh oracle for the plugin's
// provisioning script (F16 T89), wiring V81/V83/V84/V85/V88/V89/V90 into the
// standard `go test ./...` gate. Skips where /bin/sh is unavailable.
func TestEnsureSddBinaryScript(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh not available")
	}
	out, err := exec.Command("sh", "../../scripts/ensure-sdd-binary_test.sh").CombinedOutput()
	if err != nil {
		t.Fatalf("ensure-sdd-binary_test.sh failed: %v\n%s", err, out)
	}
}
