// Package elfimg extracts the ROM-bound sections of a linked ELF.
package elfimg

import (
	"debug/elf"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Section is a named block of bytes. Addr is the ROM load address for Sections and Origs, and the RAM address for NoBits.
type Section struct {
	Name string
	Addr uint64
	Data []byte
}

// Result lists what an ELF wants written, checked, or merely reported.
type Result struct {
	Sections []Section // written to the ROM at Addr
	Origs    []Section // expected original bytes at Addr; compared, never written
	NoBits   []Section // RAM-resident sections; reported only, Data is nil
}

var taggedName = regexp.MustCompile(`^\.(?:hook|orig)_([0-9A-Fa-f]{6})$`)

// Read parses the ELF at path and returns its sections with load addresses.
// A .hook_ section whose load address differs from its name is an error.
func Read(path string) (Result, error) {
	f, err := elf.Open(path)
	if err != nil {
		return Result{}, err
	}
	defer f.Close()
	if f.Class != elf.ELFCLASS32 || f.Machine != elf.EM_68K {
		return Result{}, fmt.Errorf("%s is not a 32-bit m68k ELF (class %v, machine %v)", path, f.Class, f.Machine)
	}
	var res Result
	for _, s := range f.Sections {
		if strings.HasPrefix(s.Name, ".orig_") {
			addr, err := taggedAddr(s.Name)
			if err != nil {
				return Result{}, err
			}
			data, err := s.Data()
			if err != nil {
				return Result{}, fmt.Errorf("%s: %w", s.Name, err)
			}
			res.Origs = append(res.Origs, Section{Name: s.Name, Addr: addr, Data: data})
			continue
		}
		if strings.HasPrefix(s.Name, ".hook_") {
			if s.Flags&elf.SHF_ALLOC == 0 {
				return Result{}, fmt.Errorf("%s is not allocatable; declare it with .section %s,\"ax\"", s.Name, s.Name)
			}
			if s.Size == 0 {
				return Result{}, fmt.Errorf("%s is empty", s.Name)
			}
			if s.Type == elf.SHT_NOBITS {
				return Result{}, fmt.Errorf("%s has no file contents (SHT_NOBITS); a hook must be code or data, not bss", s.Name)
			}
		}
		if s.Flags&elf.SHF_ALLOC == 0 || s.Size == 0 {
			continue
		}
		if s.Type == elf.SHT_NOBITS {
			res.NoBits = append(res.NoBits, Section{Name: s.Name, Addr: s.Addr})
			continue
		}
		load, err := loadAddr(f, s)
		if err != nil {
			return Result{}, err
		}
		if strings.HasPrefix(s.Name, ".hook_") {
			want, err := taggedAddr(s.Name)
			if err != nil {
				return Result{}, err
			}
			if want != load {
				return Result{}, fmt.Errorf("%s is linked at 0x%06X, not 0x%06X; add -Wl,--section-start=%s=0x%06X to the link", s.Name, load, want, s.Name, want)
			}
		}
		data, err := s.Data()
		if err != nil {
			return Result{}, fmt.Errorf("%s: %w", s.Name, err)
		}
		res.Sections = append(res.Sections, Section{Name: s.Name, Addr: load, Data: data})
	}
	return res, nil
}

// Unverified returns the hooks that have no .orig_ twin. An .orig_ section
// with no hook at its offset, or one whose size differs from its hook's, is an error.
func (r Result) Unverified() ([]string, error) {
	hooks := map[uint64]Section{}
	for _, s := range r.Sections {
		if strings.HasPrefix(s.Name, ".hook_") {
			hooks[s.Addr] = s
		}
	}
	paired := map[uint64]bool{}
	for _, o := range r.Origs {
		h, ok := hooks[o.Addr]
		if !ok {
			return nil, fmt.Errorf("%s has no .hook_%s twin; an .orig_ section records the bytes the hook at its offset overwrites", o.Name, strings.TrimPrefix(o.Name, ".orig_"))
		}
		if len(o.Data) != len(h.Data) {
			return nil, fmt.Errorf("%s is %d bytes but %s is %d; the twin must cover exactly the bytes the hook overwrites", h.Name, len(h.Data), o.Name, len(o.Data))
		}
		paired[o.Addr] = true
	}
	var unverified []string
	for _, s := range r.Sections {
		if strings.HasPrefix(s.Name, ".hook_") && !paired[s.Addr] {
			unverified = append(unverified, s.Name)
		}
	}
	return unverified, nil
}

// taggedAddr parses the six hex digits after .hook_ or .orig_.
func taggedAddr(name string) (uint64, error) {
	m := taggedName.FindStringSubmatch(name)
	if m == nil {
		return 0, fmt.Errorf("section %s must end in exactly six hex digits, like .hook_0051F2", name)
	}
	return strconv.ParseUint(m[1], 16, 32)
}

// loadAddr maps a section's virtual address to its load address through the PT_LOAD segment holding it.
func loadAddr(f *elf.File, s *elf.Section) (uint64, error) {
	for _, p := range f.Progs {
		if p.Type != elf.PT_LOAD {
			continue
		}
		if s.Addr >= p.Vaddr && s.Addr+s.Size <= p.Vaddr+p.Memsz {
			return p.Paddr + (s.Addr - p.Vaddr), nil
		}
	}
	return 0, fmt.Errorf("section %s at 0x%X is not covered by any PT_LOAD segment", s.Name, s.Addr)
}
