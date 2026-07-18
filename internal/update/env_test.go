package update

import (
	"os"
	"testing"
)

func TestOfflineAndSkipVersionCheck(t *testing.T) {
	t.Setenv("STELL_OFFLINE", "")
	t.Setenv("STELL_SKIP_VERSION_CHECK", "")
	if Offline() || SkipVersionCheck() {
		t.Fatal("expected checks enabled by default")
	}
	t.Setenv("STELL_SKIP_VERSION_CHECK", "1")
	if !SkipVersionCheck() {
		t.Fatal("expected skip")
	}
	if Offline() {
		t.Fatal("offline should be false")
	}
	t.Setenv("STELL_SKIP_VERSION_CHECK", "")
	t.Setenv("STELL_OFFLINE", "true")
	if !Offline() || !SkipVersionCheck() {
		t.Fatal("offline should skip version check")
	}
}

func TestIsNewer(t *testing.T) {
	if !IsNewer("0.2.0", "0.1.0") {
		t.Fatal("0.2.0 should be newer")
	}
	if IsNewer("0.1.0", "0.1.0") {
		t.Fatal("equal versions")
	}
	if !IsNewer("dev2", "dev") {
		t.Fatal("fallback string compare")
	}
}

func TestDetectInstallMethod(t *testing.T) {
	if got := DetectInstallMethod("/opt/homebrew/bin/stell"); got != InstallBrew {
		t.Fatalf("brew: got %q", got)
	}
	if got := DetectInstallMethod("/Users/me/go/bin/stell"); got != InstallGoInstall {
		t.Fatalf("go install: got %q", got)
	}
	if got := DetectInstallMethod("/Users/me/Code/Home/stell/stell"); got != InstallLocalBuild {
		t.Fatalf("local build: got %q", got)
	}
}

func TestGetSelfUpdateCommand(t *testing.T) {
	cmd, err := GetSelfUpdateCommand(InstallGoInstall, "example.com/mod/cmd", "1.2.3")
	if err != nil || cmd == nil {
		t.Fatalf("command: %v %v", cmd, err)
	}
	if cmd.Args[1] != "example.com/mod/cmd@v1.2.3" {
		t.Fatalf("args: %v", cmd.Args)
	}
	if _, err := GetSelfUpdateCommand(InstallUnknown, "", "1.0.0"); err == nil {
		t.Fatal("expected error for unknown")
	}
}

func TestEnableOffline(t *testing.T) {
	EnableOffline()
	if os.Getenv("STELL_OFFLINE") != "1" || os.Getenv("STELL_SKIP_VERSION_CHECK") != "1" {
		t.Fatal("env not set")
	}
}
