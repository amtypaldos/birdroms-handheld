package update

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// launcherBlock returns the self-update section of package/mux_launch.sh. The
// rest of that script needs muOS's /opt/muos helpers, so the test sources only
// the part between its markers and calls launch_app like the script does.
func launcherBlock(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "package", "mux_launch.sh"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	start := strings.Index(text, "# >>> self-update")
	end := strings.Index(text, "# <<< self-update")
	if start < 0 || end < start {
		t.Fatal("mux_launch.sh has no self-update block")
	}
	return text[start:end]
}

// runLauncher runs launch_app in dir with APP_BIN=birdroms and returns the
// app-run log (one line per start).
func runLauncher(t *testing.T, dir string) []string {
	t.Helper()
	script := launcherBlock(t) + "\nAPP_BIN=birdroms\nAPP_DIR=" + dir + "\ncd \"$APP_DIR\"\nlaunch_app\n"
	if out, err := exec.Command("sh", "-c", script).CombinedOutput(); err != nil {
		t.Fatalf("launcher: %v\n%s", err, out)
	}
	raw, _ := os.ReadFile(filepath.Join(dir, "runs.log"))
	return strings.Fields(string(raw))
}

// fakeApp is a birdroms that records its name, then runs extra.
func fakeApp(t *testing.T, path, name, extra string) {
	t.Helper()
	body := "#!/bin/sh\necho " + name + " >> runs.log\n" + extra + "\n"
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func TestLauncherPlainStartRunsOnce(t *testing.T) {
	dir := t.TempDir()
	fakeApp(t, filepath.Join(dir, "birdroms"), "old", "")
	if got := runLauncher(t, dir); strings.Join(got, ",") != "old" {
		t.Fatalf("runs = %v", got)
	}
}

func TestLauncherAppliesAnUpdateStagedByADeadRun(t *testing.T) {
	dir := t.TempDir()
	fakeApp(t, filepath.Join(dir, "birdroms"), "old", "")
	fakeApp(t, filepath.Join(dir, NewBinary), "new", "")
	os.WriteFile(filepath.Join(dir, NewStamp), []byte("hash2"), 0o644)

	if got := runLauncher(t, dir); strings.Join(got, ",") != "new" {
		t.Fatalf("runs = %v, want only the new app", got)
	}
	if exists(filepath.Join(dir, NewBinary)) || exists(filepath.Join(dir, NewStamp)) {
		t.Fatal("staged files were not consumed")
	}
	if got := read(t, filepath.Join(dir, StampName)); got != "hash2" {
		t.Fatalf("installed stamp = %q", got)
	}
}

func TestLauncherSwapsAfterTheAppStagesAndRestartsOnce(t *testing.T) {
	dir := t.TempDir()
	// The old app stages an update on every run, as a server that keeps changing would.
	stage := "echo '#!/bin/sh' > birdroms.new; echo 'echo new >> runs.log; echo again > birdroms.new; echo h > update.sha256' >> birdroms.new; chmod +x birdroms.new; echo hash2 > update.sha256"
	fakeApp(t, filepath.Join(dir, "birdroms"), "old", stage)

	got := runLauncher(t, dir)
	if strings.Join(got, ",") != "old,new" {
		t.Fatalf("runs = %v, want old then new once (no loop)", got)
	}
	if got := read(t, filepath.Join(dir, StampName)); strings.TrimSpace(got) != "hash2" {
		t.Fatalf("installed stamp = %q", got)
	}
}

func TestLauncherDropsAFailedUpdateAndKeepsTheOldApp(t *testing.T) {
	dir := t.TempDir()
	fakeApp(t, filepath.Join(dir, "birdroms"), "old", "")
	// A half-unpacked update: the binary is there, the stamp never was.
	fakeApp(t, filepath.Join(dir, NewBinary), "broken", "")
	os.WriteFile(filepath.Join(dir, ".update-1.part"), []byte("x"), 0o644)

	if got := runLauncher(t, dir); strings.Join(got, ",") != "old" {
		t.Fatalf("runs = %v, want the installed app", got)
	}
	for _, name := range []string{NewBinary, NewStamp, ".update-1.part"} {
		if exists(filepath.Join(dir, name)) {
			t.Fatalf("%s left behind", name)
		}
	}
	if exists(filepath.Join(dir, StampName)) {
		t.Fatal("a failed update wrote installed.sha256")
	}
}

func TestLauncherCleansUpWhenTheAppDiesMidUpdate(t *testing.T) {
	dir := t.TempDir()
	fakeApp(t, filepath.Join(dir, "birdroms"), "old", "echo partial > birdroms.new; exit 1")

	if got := runLauncher(t, dir); strings.Join(got, ",") != "old" {
		t.Fatalf("runs = %v", got)
	}
	if exists(filepath.Join(dir, NewBinary)) {
		t.Fatal("birdroms.new left behind")
	}
}
