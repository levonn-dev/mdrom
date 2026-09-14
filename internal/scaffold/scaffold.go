// Package scaffold renders the patch project that mdrom init writes beside a ROM.
package scaffold

import (
	"bytes"
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// The all: prefix keeps game/__name__.h and .ld, which embed skips by default
// because their names start with an underscore.
//
//go:embed all:templates
var templates embed.FS

// Params are the values substituted into the project templates.
type Params struct {
	Name    string // CMake project name and prefix of the game symbol files
	ROMFile string // clean ROM filename in the project root
	SHA1    string // clean ROM SHA-1, lowercase hex
	Size    int    // target image size in bytes
	Entry   uint32 // original reset vector, ROM offset 4
}

// Guard is the include guard of the game header.
func (p Params) Guard() string {
	return strings.ToUpper(p.Name) + "_H"
}

// File is one rendered project file.
type File struct {
	Path string // relative to the project root, forward slashes
	Data []byte
	Mode os.FileMode
}

// Files renders every template for p, sorted by path.
func Files(p Params) ([]File, error) {
	var files []File
	err := fs.WalkDir(templates, "templates", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		src, err := templates.ReadFile(name)
		if err != nil {
			return err
		}
		rel := strings.TrimPrefix(name, "templates/")
		tmpl, err := template.New(rel).Parse(string(src))
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, p); err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		out := strings.ReplaceAll(rel, "__name__", p.Name)
		mode := os.FileMode(0o644)
		if strings.HasPrefix(out, "scripts/") {
			mode = 0o755
		}
		files = append(files, File{Path: out, Data: buf.Bytes(), Mode: mode})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return files, nil
}

// Write renders the tree into dir. It fails before writing anything if any
// output path exists, naming the first one.
func Write(dir string, p Params) ([]File, error) {
	files, err := Files(p)
	if err != nil {
		return nil, err
	}
	for _, f := range files {
		dst := filepath.Join(dir, filepath.FromSlash(f.Path))
		// Lstat, not Stat: a dangling symlink must not slip past this check and let WriteFile write through it.
		_, err := os.Lstat(dst)
		if err == nil {
			return nil, fmt.Errorf("%s already exists", dst)
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
	}
	for _, f := range files {
		dst := filepath.Join(dir, filepath.FromSlash(f.Path))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		if err := os.WriteFile(dst, f.Data, f.Mode); err != nil {
			return nil, err
		}
		// WriteFile's mode is masked by umask, so set it explicitly.
		if err := os.Chmod(dst, f.Mode); err != nil {
			return nil, err
		}
	}
	return files, nil
}
