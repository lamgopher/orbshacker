package faker

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizeExeName(t *testing.T) {
	valid := map[string]string{
		"TslGame":               "TslGame.exe",
		"League of Legends.exe": "League of Legends.exe",
		`bin\win64\Game.EXE`:    "bin/win64/Game.EXE",
		"  Overwatch.exe  ":     "Overwatch.exe",
	}
	for in, want := range valid {
		got, err := NormalizeExeName(in)
		if err != nil || got != want {
			t.Errorf("NormalizeExeName(%q) = %q, %v; want %q", in, got, err, want)
		}
	}

	for _, in := range []string{"", "../evil.exe", "a/../../b.exe", "/abs.exe", `C:\x.exe`, "a//b.exe", "bad|name.exe"} {
		if _, err := NormalizeExeName(in); err == nil {
			t.Errorf("NormalizeExeName(%q) should fail", in)
		}
	}
}

func TestCopyFileNoOverwrite(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src")
	dst := filepath.Join(dir, "dst.exe")
	os.WriteFile(src, []byte("new"), 0o644)
	os.WriteFile(dst, []byte("real game"), 0o644)

	if err := copyFile(src, dst, false); !errors.Is(err, ErrTargetExists) {
		t.Fatalf("copyFile() error = %v, want ErrTargetExists", err)
	}
	if data, _ := os.ReadFile(dst); string(data) != "real game" {
		t.Fatalf("existing file was modified: %q", data)
	}
	if err := copyFile(src, dst, true); err != nil {
		t.Fatal(err)
	}
	if data, _ := os.ReadFile(dst); string(data) != "new" {
		t.Fatalf("overwrite failed: %q", data)
	}
}

func TestCreatedDirsCleanup(t *testing.T) {
	base := t.TempDir()
	exeDir := filepath.Join(base, "common", "Game", "bin")

	root := firstMissingDir(exeDir)
	if root != filepath.Join(base, "common") {
		t.Fatalf("firstMissingDir() = %q", root)
	}
	if err := os.MkdirAll(exeDir, 0o755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(exeDir, "Game.exe")
	os.WriteFile(exe, []byte("x"), 0o644)

	f := &Fake{ExePath: exe, createdRoot: root}
	if err := f.RemoveFiles(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(root); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("created root should be removed, stat err = %v", err)
	}
	if _, err := os.Stat(base); err != nil {
		t.Fatalf("pre-existing dir must stay: %v", err)
	}
}

func TestRemoveEmptyDirsKeepsNonEmpty(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "Win64")
	os.MkdirAll(root, 0o755)
	os.WriteFile(filepath.Join(root, "other.exe"), []byte("x"), 0o644)

	removeEmptyDirs(root, root)
	if _, err := os.Stat(root); err != nil {
		t.Fatalf("non-empty dir must stay: %v", err)
	}
}

func TestRemoveFilesKeepsPreexistingExe(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "Game.exe")
	manifest := filepath.Join(dir, "appmanifest_1.acf")
	os.WriteFile(exe, []byte("real game"), 0o644)
	os.WriteFile(manifest, []byte("x"), 0o644)

	f := &Fake{ExePath: exe, ManifestPath: manifest, keepExe: true}
	if err := f.RemoveFiles(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(exe); err != nil {
		t.Fatalf("pre-existing exe must stay: %v", err)
	}
	if _, err := os.Stat(manifest); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("manifest should be removed, stat err = %v", err)
	}
}
