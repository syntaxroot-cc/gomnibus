package health

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// ── parseDeps ─────────────────────────────────────────────────────────────────

func TestParseDeps_LDD(t *testing.T) {
	output := `	linux-vdso.so.1 (0x00007ffd00000000)
	libssl.so.1.1 => /lib/x86_64-linux-gnu/libssl.so.1.1 (0x00007f0000000000)
	libcrypto.so.1.1 => /lib/x86_64-linux-gnu/libcrypto.so.1.1 (0x00007f0000001000)
	libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007f0000002000)
	/lib64/ld-linux-x86-64.so.2 (0x00007f0000003000)`

	deps := parseDeps(output)
	found := make(map[string]bool)
	for _, d := range deps {
		found[d] = true
	}

	if !found["/lib/x86_64-linux-gnu/libssl.so.1.1"] {
		t.Errorf("libssl not in deps: %v", deps)
	}
	if !found["/lib/x86_64-linux-gnu/libc.so.6"] {
		t.Errorf("libc not in deps: %v", deps)
	}
	// linux-vdso is not an absolute path — should not be included
	for _, d := range deps {
		if d == "linux-vdso.so.1" {
			t.Error("vdso should not be included")
		}
	}
}

func TestParseDeps_Otool(t *testing.T) {
	output := `/usr/local/bin/openssl:
	/usr/local/lib/libssl.3.dylib (compatibility version 3.0.0, current version 3.0.0)
	/usr/local/lib/libcrypto.3.dylib (compatibility version 3.0.0, current version 3.0.0)
	/usr/lib/libSystem.B.dylib (compatibility version 1.0.0, current version 1311.0.0)`

	deps := parseDeps(output)
	found := make(map[string]bool)
	for _, d := range deps {
		found[d] = true
	}

	if !found["/usr/local/lib/libssl.3.dylib"] {
		t.Errorf("libssl not in otool deps: %v", deps)
	}
	if !found["/usr/lib/libSystem.B.dylib"] {
		t.Errorf("libSystem not in otool deps: %v", deps)
	}
}

func TestParseDeps_Empty(t *testing.T) {
	if deps := parseDeps(""); len(deps) != 0 {
		t.Errorf("empty input should produce no deps, got %v", deps)
	}
}

func TestParseDeps_NoAbsolutePaths(t *testing.T) {
	output := `	not-found (0x0)
	statically-linked`
	deps := parseDeps(output)
	if len(deps) != 0 {
		t.Errorf("expected no deps without absolute paths, got %v", deps)
	}
}

// ── isWhitelisted ─────────────────────────────────────────────────────────────

func TestIsWhitelisted(t *testing.T) {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`/lib/x86_64-linux-gnu/.*`),
		regexp.MustCompile(`/usr/lib/.*`),
	}
	cases := []struct {
		lib  string
		want bool
	}{
		{"/lib/x86_64-linux-gnu/libc.so.6", true},
		{"/usr/lib/libSystem.dylib", true},
		{"/opt/myapp/lib/libssl.so", false},
		{"/home/user/mylib.so", false},
		{"/lib/not-matched.so", false},
	}
	for _, c := range cases {
		got := isWhitelisted(c.lib, patterns)
		if got != c.want {
			t.Errorf("isWhitelisted(%q) = %v, want %v", c.lib, got, c.want)
		}
	}
}

func TestIsWhitelisted_NoPatterns(t *testing.T) {
	if isWhitelisted("/usr/lib/foo.so", nil) {
		t.Error("expected false with no patterns")
	}
}

// ── isELFOrMachO ─────────────────────────────────────────────────────────────

func TestIsELFOrMachO_ELF(t *testing.T) {
	path := writeBytes(t, "elf", []byte{0x7f, 'E', 'L', 'F', 0x02, 0x01, 0x01, 0x00})
	if !isELFOrMachO(path) {
		t.Error("ELF magic not detected")
	}
}

func TestIsELFOrMachO_MachO64LE(t *testing.T) {
	path := writeBytes(t, "macho64le", []byte{0xCF, 0xFA, 0xED, 0xFE, 0x0C, 0x00, 0x00, 0x01})
	if !isELFOrMachO(path) {
		t.Error("Mach-O 64-bit LE not detected")
	}
}

func TestIsELFOrMachO_MachO64BE(t *testing.T) {
	path := writeBytes(t, "macho64be", []byte{0xFE, 0xED, 0xFA, 0xCF, 0x00, 0x00, 0x00, 0x0C})
	if !isELFOrMachO(path) {
		t.Error("Mach-O 64-bit BE not detected")
	}
}

func TestIsELFOrMachO_MachOFat(t *testing.T) {
	path := writeBytes(t, "machoFat", []byte{0xCA, 0xFE, 0xBA, 0xBE, 0x00, 0x00, 0x00, 0x02})
	if !isELFOrMachO(path) {
		t.Error("Mach-O fat binary not detected")
	}
}

func TestIsELFOrMachO_TextFile(t *testing.T) {
	path := writeBytes(t, "text.txt", []byte("#!/bin/sh\necho hello\n"))
	if isELFOrMachO(path) {
		t.Error("text file should not match binary magic")
	}
}

func TestIsELFOrMachO_TooShort(t *testing.T) {
	path := writeBytes(t, "short", []byte{0x7f, 'E'})
	if isELFOrMachO(path) {
		t.Error("too-short file should not match")
	}
}

func TestIsELFOrMachO_Empty(t *testing.T) {
	path := writeBytes(t, "empty", []byte{})
	if isELFOrMachO(path) {
		t.Error("empty file should not match")
	}
}

func TestIsELFOrMachO_NotFound(t *testing.T) {
	if isELFOrMachO(filepath.Join(t.TempDir(), "nonexistent")) {
		t.Error("non-existent file should return false")
	}
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeBytes(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
