// Package health verifies that the install root does not link against unexpected
// shared libraries — mirroring the Omnibus healthcheck system.
package health

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// CheckOptions configures the health check pass.
type CheckOptions struct {
	InstallDir     string
	WhitelistFiles []*regexp.Regexp
	Log            *zap.Logger
}

// Check walks the install root looking for binaries that depend on shared
// libraries outside the install root. It reports violations unless they are
// covered by a whitelist pattern.
func Check(opts CheckOptions) error {
	var violations []string

	err := filepath.Walk(opts.InstallDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		if !isELFOrMachO(path) {
			return nil
		}
		deps, err := sharedLibs(path)
		if err != nil {
			return nil // skip files we can't inspect
		}
		for _, dep := range deps {
			if strings.HasPrefix(dep, opts.InstallDir) {
				continue
			}
			if isWhitelisted(dep, opts.WhitelistFiles) {
				opts.Log.Debug("whitelisted library", zap.String("lib", dep), zap.String("binary", path))
				continue
			}
			violations = append(violations, fmt.Sprintf("%s links to external library %s", path, dep))
		}
		return nil
	})
	if err != nil {
		return err
	}
	if len(violations) > 0 {
		return fmt.Errorf("health check failed:\n  %s", strings.Join(violations, "\n  "))
	}
	return nil
}

func sharedLibs(path string) ([]string, error) {
	// ldd on Linux, otool on macOS.
	var out []byte
	var err error
	switch {
	case commandExists("ldd"):
		out, err = exec.Command("ldd", path).CombinedOutput()
	case commandExists("otool"):
		out, err = exec.Command("otool", "-L", path).CombinedOutput()
	default:
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return parseDeps(string(out)), nil
}

func parseDeps(output string) []string {
	var deps []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "linux-") || strings.HasPrefix(line, "ld-") {
			continue
		}
		// ldd format: "libfoo.so.1 => /lib/libfoo.so.1 (0x...)"
		// otool format: "\t/usr/lib/libfoo.dylib (compatibility ...)"
		parts := strings.Fields(line)
		for _, p := range parts {
			if strings.HasPrefix(p, "/") {
				deps = append(deps, p)
				break
			}
		}
	}
	return deps
}

func isWhitelisted(lib string, patterns []*regexp.Regexp) bool {
	for _, re := range patterns {
		if re.MatchString(lib) {
			return true
		}
	}
	return false
}

func isELFOrMachO(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	magic := make([]byte, 4)
	if _, err := f.Read(magic); err != nil {
		return false
	}
	// ELF magic: 0x7f 'E' 'L' 'F'
	if magic[0] == 0x7f && magic[1] == 'E' && magic[2] == 'L' && magic[3] == 'F' {
		return true
	}
	// Mach-O magic numbers (little-endian and big-endian, 32/64 bit, fat)
	macho := [][]byte{
		{0xCF, 0xFA, 0xED, 0xFE},
		{0xCE, 0xFA, 0xED, 0xFE},
		{0xFE, 0xED, 0xFA, 0xCF},
		{0xFE, 0xED, 0xFA, 0xCE},
		{0xCA, 0xFE, 0xBA, 0xBE},
	}
	for _, m := range macho {
		if magic[0] == m[0] && magic[1] == m[1] && magic[2] == m[2] && magic[3] == m[3] {
			return true
		}
	}
	return false
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}
