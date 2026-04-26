package plugin_manager

import (
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestCommandFromVerifiedRuntimeFileExecsBinary(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("verified-fd exec test requires linux")
	}

	echoPath, err := exec.LookPath("echo")
	if err != nil {
		t.Fatalf("exec.LookPath(\"echo\") error = %v", err)
	}

	raw, err := os.ReadFile(echoPath)
	if err != nil {
		t.Fatalf("os.ReadFile(%q) error = %v", echoPath, err)
	}

	verifiedFile, err := verifyRuntimeBinaryChecksum(echoPath, fmt.Sprintf("%x", sha256.Sum256(raw)))
	if err != nil {
		t.Fatalf("verifyRuntimeBinaryChecksum() error = %v", err)
	}
	defer verifiedFile.Close()

	cmd := commandFromVerifiedRuntimeFile(verifiedFile)
	cmd.Args = append(cmd.Args, "hello-from-fd")

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cmd.CombinedOutput() error = %v, output=%s", err, out)
	}
	if got, want := strings.TrimSpace(string(out)), "hello-from-fd"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
