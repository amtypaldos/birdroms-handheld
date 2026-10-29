// Package update decides when the published handheld package is newer than
// the one installed, and stages that package beside the running binary.
// mux_launch.sh swaps the new binary in after this process exits, because a
// running executable cannot replace itself.
package update

import (
	"archive/zip"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/amtypaldos/birdroms-handheld/internal/api"
	"github.com/amtypaldos/birdroms-handheld/internal/download"
)

const (
	// StampName is the SHA-256 of the package the server reported, not of the
	// downloaded zip (that zip also contains this server's server.json).
	StampName = "installed.sha256"
	// NewBinary is the staged executable. The launcher renames it over birdroms.
	NewBinary = "birdroms.new"
	// NewStamp is the staged hash, renamed to StampName only after the binary swap.
	NewStamp = "update.sha256"

	zipRoot = "birdROMs/"
)

// NeedsUpdate reports whether the paired server has a package this install
// does not already have. A missing local stamp means "download once".
func NeedsUpdate(installed, remote string, available bool) bool {
	remote = strings.TrimSpace(remote)
	if !available || remote == "" {
		return false
	}
	return strings.TrimSpace(installed) != remote
}

// ReadStamp returns the recorded package hash, or "" when the file is absent.
func ReadStamp(dir string) (string, error) {
	raw, err := os.ReadFile(filepath.Join(dir, StampName))
	if os.IsNotExist(err) {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(raw)), nil
}

// Ready reports whether a staged update is waiting for the launcher.
func Ready(dir string) bool {
	if _, err := os.Stat(filepath.Join(dir, NewBinary)); err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(dir, NewStamp)); err != nil {
		return false
	}
	return true
}

// Stage unpacks a .muxapp into dir. The executable is written to birdroms.new.
// config.json is never created or replaced. A failure removes birdroms.new.
func Stage(r io.ReaderAt, size int64, dir string) (err error) {
	archive, err := zip.NewReader(r, size)
	if err != nil {
		return fmt.Errorf("update: not a package: %w", err)
	}
	defer func() {
		if err != nil {
			_ = os.Remove(filepath.Join(dir, NewBinary))
		}
	}()
	// Check every entry before writing any, so a bad package changes nothing.
	sawBinary := false
	for _, file := range archive.File {
		name := filepath.ToSlash(file.Name)
		if strings.HasSuffix(name, "/") {
			continue
		}
		rel, ok := entryPath(name)
		if !ok {
			return fmt.Errorf("update: refusing entry %q", file.Name)
		}
		if rel == "birdroms" {
			sawBinary = true
		}
	}
	if !sawBinary {
		return fmt.Errorf("update: package has no birdROMs/birdroms")
	}
	for _, file := range archive.File {
		name := filepath.ToSlash(file.Name)
		if strings.HasSuffix(name, "/") {
			continue
		}
		rel, _ := entryPath(name)
		if rel == "config.json" {
			continue
		}
		dest, mode := filepath.Join(dir, rel), os.FileMode(0o644)
		if rel == "birdroms" {
			dest, mode = filepath.Join(dir, NewBinary), 0o755
		} else if strings.HasSuffix(rel, ".sh") {
			mode = 0o755
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return err
		}
		if err := writeZipFile(file, dest, mode); err != nil {
			return err
		}
	}
	return nil
}

func entryPath(name string) (string, bool) {
	if !strings.HasPrefix(name, zipRoot) {
		return "", false
	}
	rel := strings.TrimPrefix(name, zipRoot)
	if rel == "" || strings.Contains(rel, "\\") {
		return "", false
	}
	clean := filepath.ToSlash(filepath.Clean(rel))
	if clean == ".." || strings.HasPrefix(clean, "../") || filepath.IsAbs(rel) {
		return "", false
	}
	return clean, true
}

// writeZipFile writes to a temporary name and renames it into place. The
// rename gives dest a new inode, so a shell that is still reading the old
// mux_launch.sh (it is waiting for this process) keeps reading the old text.
func writeZipFile(file *zip.File, dest string, mode os.FileMode) error {
	in, err := file.Open()
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dest + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr == nil {
		copyErr = closeErr
	}
	if copyErr == nil {
		copyErr = os.Rename(tmp, dest)
	}
	if copyErr != nil {
		_ = os.Remove(tmp)
	}
	return copyErr
}

// Downloader fetches the personalized package. *api.Client implements it.
type Downloader interface {
	DownloadHandheld(ctx context.Context, w io.Writer, progress func(written int64)) error
}

// Job is one app update, shaped like a download queue so the game download
// screen can draw it.
type Job struct {
	dl   Downloader
	dir  string
	sha  string
	size int64

	mu       sync.Mutex
	item     download.Item
	finished bool
}

// NewJob prepares an update of the package whose server-reported hash is sha.
func NewJob(dl Downloader, dir, sha string, size int64) *Job {
	return &Job{
		dl:   dl,
		dir:  dir,
		sha:  sha,
		size: size,
		item: download.Item{
			ROM:    api.ROM{ID: 1, Title: "Updating birdROMs", Platform: "App"},
			Status: download.Pending,
			Total:  size,
			Known:  true, // no platform folder to warn about
		},
	}
}

// Snapshot returns the single item for the download screen.
func (j *Job) Snapshot() []download.Item {
	j.mu.Lock()
	defer j.mu.Unlock()
	return []download.Item{j.item}
}

// Finished reports whether Run has returned.
func (j *Job) Finished() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.finished
}

func (j *Job) set(fn func(*download.Item)) {
	j.mu.Lock()
	fn(&j.item)
	j.mu.Unlock()
}

// Run downloads the package and stages it. Cancel or any error removes the
// staged binary so the launcher keeps the copy that is already installed.
func (j *Job) Run(ctx context.Context) {
	defer func() {
		j.mu.Lock()
		j.finished = true
		j.mu.Unlock()
	}()
	j.set(func(it *download.Item) { it.Status = download.Active })

	part, err := os.CreateTemp(j.dir, ".update-*.part")
	if err != nil {
		j.fail(err.Error())
		return
	}
	partName := part.Name()
	removePart := true
	defer func() {
		if removePart {
			_ = os.Remove(partName)
			_ = os.Remove(filepath.Join(j.dir, NewBinary))
			_ = os.Remove(filepath.Join(j.dir, NewStamp))
		}
	}()

	err = j.dl.DownloadHandheld(ctx, part, func(n int64) {
		j.set(func(it *download.Item) { it.Written = n })
	})
	if closeErr := part.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		if ctx.Err() != nil {
			j.set(func(it *download.Item) { it.Status = download.Cancelled })
			return
		}
		j.fail(err.Error())
		return
	}

	info, err := os.Stat(partName)
	if err != nil {
		j.fail(err.Error())
		return
	}
	file, err := os.Open(partName)
	if err != nil {
		j.fail(err.Error())
		return
	}
	stageErr := Stage(file, info.Size(), j.dir)
	file.Close()
	if stageErr != nil {
		j.fail(stageErr.Error())
		return
	}
	if err := os.WriteFile(filepath.Join(j.dir, NewStamp), []byte(j.sha), 0o644); err != nil {
		j.fail(err.Error())
		return
	}
	removePart = false
	_ = os.Remove(partName)
	j.set(func(it *download.Item) {
		it.Status = download.Done
		if it.Total > 0 {
			it.Written = it.Total
		}
	})
}

func (j *Job) fail(msg string) {
	j.set(func(it *download.Item) { it.Status, it.Err = download.Failed, msg })
}
