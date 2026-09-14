package scaffold

import (
	"bytes"
	"flag"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite testdata/golden from the templates")

var goldenParams = Params{
	Name:    "synth",
	ROMFile: "base.md",
	SHA1:    "0123456789abcdef0123456789abcdef01234567",
	Size:    0x200000,
	Entry:   0x200,
}

const goldenDir = "testdata/golden"

func TestFilesMatchGolden(t *testing.T) {
	files, err := Files(goldenParams)
	if err != nil {
		t.Fatal(err)
	}
	if *update {
		if err := os.RemoveAll(goldenDir); err != nil {
			t.Fatal(err)
		}
		for _, f := range files {
			dst := filepath.Join(goldenDir, filepath.FromSlash(f.Path))
			if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(dst, f.Data, f.Mode); err != nil {
				t.Fatal(err)
			}
		}
	}
	rendered := map[string]bool{}
	for _, f := range files {
		rendered[f.Path] = true
		want, err := os.ReadFile(filepath.Join(goldenDir, filepath.FromSlash(f.Path)))
		if err != nil {
			t.Errorf("%s: %v (run go test ./internal/scaffold -update if intended)", f.Path, err)
			continue
		}
		if !bytes.Equal(f.Data, want) {
			t.Errorf("%s differs from %s (run go test ./internal/scaffold -update if intended)", f.Path, goldenDir)
		}
	}
	err = filepath.WalkDir(goldenDir, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(goldenDir, p)
		if err != nil {
			return err
		}
		if !rendered[filepath.ToSlash(rel)] {
			t.Errorf("%s is in %s but is not rendered", rel, goldenDir)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestFilesAreFullyRendered(t *testing.T) {
	files, err := Files(goldenParams)
	if err != nil {
		t.Fatal(err)
	}
	if len(files) != 15 {
		t.Errorf("%d files rendered, want 15", len(files))
	}
	for _, f := range files {
		if bytes.Contains(f.Data, []byte("{{")) {
			t.Errorf("%s still contains a template action", f.Path)
		}
		if bytes.Contains([]byte(f.Path), []byte("__name__")) {
			t.Errorf("%s still contains the __name__ placeholder", f.Path)
		}
	}
}

func TestFileModes(t *testing.T) {
	files, err := Files(goldenParams)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		want := os.FileMode(0o644)
		if filepath.Dir(f.Path) == "scripts" {
			want = 0o755
		}
		if f.Mode != want {
			t.Errorf("%s mode %o, want %o", f.Path, f.Mode, want)
		}
	}
}

func TestFilesSubstituteParams(t *testing.T) {
	files, err := Files(goldenParams)
	if err != nil {
		t.Fatal(err)
	}
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = string(f.Data)
	}
	for _, c := range []struct{ path, want string }{
		{"CMakeLists.txt", "project(synth C ASM)"},
		{"CMakeLists.txt", `set(CLEAN_ROM "${CMAKE_SOURCE_DIR}/base.md" CACHE FILEPATH "Clean ROM image")`},
		{"CMakeLists.txt", "set(CLEAN_ROM_SHA1 0123456789abcdef0123456789abcdef01234567)"},
		{"CMakeLists.txt", "set(ROM_SIZE 2097152) # 0x200000"},
		{"CMakeLists.txt", "${CMAKE_SOURCE_DIR}/game/synth.ld"},
		{"hooks/hooks.s", ".long   0x00000200"},
		{"hooks/hooks.s", "jmp     0x200 "},
		{"game/synth.h", "#ifndef SYNTH_H"},
		{"game/synth.h", "#define SYNTH_H"},
		{"link/rom.ld", "INCLUDE synth.ld"},
		{"src/patch.c", `#include "synth.h"`},
	} {
		if !strings.Contains(byPath[c.path], c.want) {
			t.Errorf("%s lacks %q", c.path, c.want)
		}
	}
	if _, ok := byPath["game/synth.ld"]; !ok {
		t.Error("game/synth.ld not rendered")
	}
}

func TestWriteRefusesExistingFile(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "hooks"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "hooks", "hooks.s"), []byte("mine\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Write(dir, goldenParams)
	if err == nil || !bytes.Contains([]byte(err.Error()), []byte(filepath.Join("hooks", "hooks.s")+" already exists")) {
		t.Fatalf("err = %v, want hooks/hooks.s already exists", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "CMakeLists.txt")); err == nil {
		t.Error("CMakeLists.txt was written despite the refusal")
	}
	if data, _ := os.ReadFile(filepath.Join(dir, "hooks", "hooks.s")); string(data) != "mine\n" {
		t.Error("the existing hooks.s was changed")
	}
}

func TestWriteRefusesDanglingSymlink(t *testing.T) {
	dir := t.TempDir()
	link := filepath.Join(dir, "CMakeLists.txt")
	if err := os.Symlink(filepath.Join(dir, "nonexistent-target"), link); err != nil {
		// Some filesystems (e.g. FAT) cannot hold symlinks; skip rather than fail there.
		t.Skipf("symlinks unsupported: %v", err)
	}
	_, err := Write(dir, goldenParams)
	if err == nil || !strings.Contains(err.Error(), "CMakeLists.txt already exists") {
		t.Fatalf("err = %v, want CMakeLists.txt already exists", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "hooks", "hooks.s")); err == nil {
		t.Error("hooks/hooks.s was written despite the refusal")
	}
}

func TestWriteCreatesTree(t *testing.T) {
	dir := t.TempDir()
	files, err := Write(dir, goldenParams)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		info, err := os.Stat(filepath.Join(dir, filepath.FromSlash(f.Path)))
		if err != nil {
			t.Errorf("%s: %v", f.Path, err)
			continue
		}
		if info.Mode().Perm() != f.Mode {
			t.Errorf("%s on disk has mode %o, want %o", f.Path, info.Mode().Perm(), f.Mode)
		}
	}
}
