// Package faker creates and launches fake game processes.
//
// A fake process is a copy of the orbshacker binary renamed to the executable
// name Discord expects. It is started with --timer-mode and shows a countdown.
package faker

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// TimerModeFlag makes the binary run as a fake game process.
const TimerModeFlag = "--timer-mode"

// TitleFlag passes the game name to the timer window.
const TitleFlag = "--title"

// ErrTargetExists is returned when the target file exists and overwriting is not allowed.
var ErrTargetExists = errors.New("target file already exists")

// Fake is a launched fake game process together with the files created for it.
type Fake struct {
	ID      int
	Game    string
	ExePath string
	// ManifestPath is set for Steam Quest Mode fakes.
	ManifestPath string
	PID          int
	Started      time.Time
	Exited       bool
	ExitedAt     time.Time
	// CleanupPending marks a fake whose files must be deleted once the process exits.
	CleanupPending bool

	// createdRoot is the topmost directory created for the exe, "" if none was created.
	createdRoot string
	// keepExe is set when the exe existed before launch and must survive cleanup.
	keepExe bool
	cmd     *exec.Cmd
}

// Running reports whether the process is still alive.
func (f *Fake) Running() bool { return !f.Exited }

// Manager keeps track of launched fakes.
type Manager struct {
	sourceExe string
	fakeDir   string
	fakes     []*Fake
	nextID    int
}

// NewManager creates a manager that puts desktop fakes into Desktop/<fakeDirName>.
func NewManager(fakeDirName string) (*Manager, error) {
	src, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("locate own executable: %w", err)
	}
	return &Manager{
		sourceExe: src,
		fakeDir:   filepath.Join(desktopDir(), fakeDirName),
		nextID:    1,
	}, nil
}

// DesktopTarget returns where a desktop fake with the given executable name is created.
func (m *Manager) DesktopTarget(exeName string) (string, error) {
	rel, err := NormalizeExeName(exeName)
	if err != nil {
		return "", err
	}
	return filepath.Join(m.fakeDir, filepath.FromSlash(rel)), nil
}

// Fakes returns all tracked fakes in launch order.
func (m *Manager) Fakes() []*Fake { return m.fakes }

// Get returns a fake by ID.
func (m *Manager) Get(id int) *Fake {
	for _, f := range m.fakes {
		if f.ID == id {
			return f
		}
	}
	return nil
}

// RunningCount returns the number of fakes whose process is alive.
func (m *Manager) RunningCount() int {
	n := 0
	for _, f := range m.fakes {
		if f.Running() {
			n++
		}
	}
	return n
}

// LaunchDesktop creates Desktop/<dir>/<exeName> and starts it.
func (m *Manager) LaunchDesktop(game, exeName string) (*Fake, error) {
	target, err := m.DesktopTarget(exeName)
	if err != nil {
		return nil, err
	}
	if m.runningAt(target) {
		return nil, fmt.Errorf("%s is already running", filepath.Base(target))
	}
	return m.launch(game, target, "", true)
}

// LaunchAt creates the fake exe at an exact path and starts it. An existing file
// is replaced only when overwrite is set. manifestPath is remembered so cleanup
// removes it too.
func (m *Manager) LaunchAt(game, target, manifestPath string, overwrite bool) (*Fake, error) {
	return m.launch(game, target, manifestPath, overwrite)
}

// StartExisting starts the executable already present at target without
// replacing it. Cleanup removes the manifest but keeps the executable.
func (m *Manager) StartExisting(game, target, manifestPath string) (*Fake, error) {
	cmd, err := start(game, target)
	if err != nil {
		return nil, err
	}
	f := m.track(game, target, manifestPath, cmd)
	f.keepExe = true
	return f, nil
}

// IsRunning reports whether a tracked fake is running from target.
func (m *Manager) IsRunning(target string) bool {
	return m.runningAt(target)
}

func (m *Manager) launch(game, target, manifestPath string, overwrite bool) (*Fake, error) {
	root := firstMissingDir(filepath.Dir(target))
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return nil, fmt.Errorf("create folder: %w", err)
	}
	if err := copyFile(m.sourceExe, target, overwrite); err != nil {
		removeEmptyDirs(filepath.Dir(target), root)
		return nil, err
	}

	cmd, err := start(game, target)
	if err != nil {
		os.Remove(target)
		removeEmptyDirs(filepath.Dir(target), root)
		return nil, err
	}
	f := m.track(game, target, manifestPath, cmd)
	f.createdRoot = root
	return f, nil
}

func start(game, target string) (*exec.Cmd, error) {
	cmd := exec.Command(target, TimerModeFlag, TitleFlag, game)
	cmd.Dir = filepath.Dir(target)
	cmd.SysProcAttr = detachedAttr()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}
	return cmd, nil
}

func (m *Manager) track(game, target, manifestPath string, cmd *exec.Cmd) *Fake {
	f := &Fake{
		ID:           m.nextID,
		Game:         game,
		ExePath:      target,
		ManifestPath: manifestPath,
		PID:          cmd.Process.Pid,
		Started:      time.Now(),
		cmd:          cmd,
	}
	m.nextID++
	m.fakes = append(m.fakes, f)
	return f
}

// Wait blocks until the fake process exits. Call it from a background goroutine.
func (f *Fake) Wait() error {
	return f.cmd.Wait()
}

// MarkExited records that the process has exited.
func (f *Fake) MarkExited() {
	f.Exited = true
	f.ExitedAt = time.Now()
}

// Stop kills the process. It is a no-op for an exited process.
func (f *Fake) Stop() error {
	if f.Exited {
		return nil
	}
	if err := f.cmd.Process.Kill(); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return err
	}
	return nil
}

// RemoveFiles deletes the exe, the manifest and directories created for the fake.
// An exe that existed before launch is kept. The process must have exited.
// Windows may keep the image locked for a moment after exit, so deletion is
// retried briefly.
func (f *Fake) RemoveFiles() error {
	var errs []error
	if !f.keepExe {
		if err := removeWithRetry(f.ExePath); err != nil {
			errs = append(errs, err)
		}
	}
	if f.ManifestPath != "" {
		if err := os.Remove(f.ManifestPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			errs = append(errs, err)
		}
	}
	removeEmptyDirs(filepath.Dir(f.ExePath), f.createdRoot)
	return errors.Join(errs...)
}

// Forget removes a fake from the list.
func (m *Manager) Forget(id int) {
	for i, f := range m.fakes {
		if f.ID == id {
			m.fakes = append(m.fakes[:i], m.fakes[i+1:]...)
			return
		}
	}
}

func (m *Manager) runningAt(target string) bool {
	for _, f := range m.fakes {
		if f.Running() && strings.EqualFold(f.ExePath, target) {
			return true
		}
	}
	return false
}

// NormalizeExeName validates a relative executable path coming from the API or
// the user and appends ".exe" when missing. The result uses forward slashes.
func NormalizeExeName(name string) (string, error) {
	name = strings.TrimSpace(strings.ReplaceAll(name, `\`, "/"))
	if name == "" {
		return "", errors.New("executable name is empty")
	}
	if !strings.HasSuffix(strings.ToLower(name), ".exe") {
		name += ".exe"
	}
	if err := ValidateRelPath(name); err != nil {
		return "", err
	}
	return name, nil
}

// ValidateRelPath checks that p is a relative path that stays inside its base folder.
func ValidateRelPath(p string) error {
	p = strings.ReplaceAll(p, `\`, "/")
	if p == "" {
		return errors.New("path is empty")
	}
	if strings.HasPrefix(p, "/") || filepath.VolumeName(filepath.FromSlash(p)) != "" {
		return fmt.Errorf("path must be relative: %q", p)
	}
	for _, part := range strings.Split(p, "/") {
		if part == "" || part == "." || part == ".." {
			return fmt.Errorf("invalid path: %q", p)
		}
		if strings.ContainsAny(part, `<>:"|?*`) {
			return fmt.Errorf("invalid characters in path: %q", p)
		}
	}
	return nil
}

func copyFile(src, dst string, overwrite bool) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open source executable: %w", err)
	}
	defer in.Close()

	flags := os.O_WRONLY | os.O_CREATE | os.O_TRUNC
	if !overwrite {
		flags = os.O_WRONLY | os.O_CREATE | os.O_EXCL
	}
	out, err := os.OpenFile(dst, flags, 0o755)
	if errors.Is(err, os.ErrExist) {
		return fmt.Errorf("%w: %s", ErrTargetExists, dst)
	}
	if err != nil {
		return fmt.Errorf("create executable: %w", err)
	}
	_, err = io.Copy(out, in)
	if closeErr := out.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		os.Remove(dst)
		return fmt.Errorf("copy executable: %w", err)
	}
	return nil
}

// firstMissingDir returns the topmost ancestor of dir (inclusive) that does not exist yet.
func firstMissingDir(dir string) string {
	missing := ""
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		if _, err := os.Stat(d); err == nil {
			return missing
		}
		missing = d
		if parent := filepath.Dir(d); parent == d {
			return missing
		}
	}
}

// removeEmptyDirs removes empty directories from dir up to and including root.
func removeEmptyDirs(dir, root string) {
	if root == "" {
		return
	}
	root = filepath.Clean(root)
	for d := filepath.Clean(dir); ; d = filepath.Dir(d) {
		rel, err := filepath.Rel(root, d)
		if err != nil || strings.HasPrefix(rel, "..") {
			return
		}
		if os.Remove(d) != nil || d == root {
			return
		}
	}
}

func removeWithRetry(path string) error {
	var err error
	for i := 0; i < 10; i++ {
		err = os.Remove(path)
		if err == nil || errors.Is(err, os.ErrNotExist) {
			return nil
		}
		time.Sleep(200 * time.Millisecond)
	}
	return err
}
