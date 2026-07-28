package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	cfg := Default()
	if cfg.Workers != 4 {
		t.Errorf("Workers: got %d, want 4", cfg.Workers)
	}
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want info", cfg.LogLevel)
	}
	if !cfg.UseGitCaching {
		t.Error("UseGitCaching: want true")
	}
	if len(cfg.SoftwareDirs) == 0 {
		t.Error("SoftwareDirs: want at least one default entry")
	}
	if cfg.BaseDir == "" {
		t.Error("BaseDir: want non-empty default")
	}
}

func TestLoad_MissingFile(t *testing.T) {
	cfg, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	if err != nil {
		t.Fatalf("Load missing file should return defaults, got: %v", err)
	}
	if cfg.Workers != 4 {
		t.Errorf("Workers default not applied: got %d", cfg.Workers)
	}
}

func TestLoad_ParsesAllFields(t *testing.T) {
	path := writeTempYAML(t, `
workers: 8
log_level: debug
s3_bucket: my-bucket
s3_region: us-east-1
s3_access_key: AKIATEST
s3_secret_key: secretval
s3_iam_role_arn: arn:aws:iam::123:role/test
s3_prefix: builds/
use_s3_caching: true
use_git_caching: false
append_timestamp: true
base_dir: /tmp/gomnibus/build
cache_dir: /tmp/gomnibus/cache
software_dirs:
  - config/software
  - extra/software
`)
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	check := func(label string, got, want interface{}) {
		t.Helper()
		if got != want {
			t.Errorf("%s: got %v, want %v", label, got, want)
		}
	}
	check("Workers", cfg.Workers, 8)
	check("LogLevel", cfg.LogLevel, "debug")
	check("S3Bucket", cfg.S3Bucket, "my-bucket")
	check("S3Region", cfg.S3Region, "us-east-1")
	check("S3AccessKey", cfg.S3AccessKey, "AKIATEST")
	check("S3SecretKey", cfg.S3SecretKey, "secretval")
	check("S3IAMRoleARN", cfg.S3IAMRoleARN, "arn:aws:iam::123:role/test")
	check("S3Prefix", cfg.S3Prefix, "builds/")
	check("UseS3Caching", cfg.UseS3Caching, true)
	check("UseGitCaching", cfg.UseGitCaching, false)
	check("AppendTimestamp", cfg.AppendTimestamp, true)
	check("BaseDir", cfg.BaseDir, "/tmp/gomnibus/build")
	check("CacheDir", cfg.CacheDir, "/tmp/gomnibus/cache")
	if len(cfg.SoftwareDirs) != 2 {
		t.Errorf("SoftwareDirs: got %v", cfg.SoftwareDirs)
	}
}

func TestLoad_OverridesDefaults(t *testing.T) {
	path := writeTempYAML(t, "workers: 1\nuse_git_caching: false\n")
	cfg, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	if cfg.Workers != 1 {
		t.Errorf("Workers: got %d, want 1", cfg.Workers)
	}
	if cfg.UseGitCaching {
		t.Error("UseGitCaching: want false after override")
	}
	// Unspecified fields keep the default
	if cfg.LogLevel != "info" {
		t.Errorf("LogLevel: got %q, want info (default)", cfg.LogLevel)
	}
}

func TestLoad_BadYAML(t *testing.T) {
	path := writeTempYAML(t, "workers: [unclosed\n")
	_, err := Load(path)
	if err == nil {
		t.Error("expected error for malformed YAML")
	}
}

func TestLoad_EmptyPath_UsesDefault(t *testing.T) {
	// With an empty path Load tries "gomnibus.yaml" in the cwd.
	// If it doesn't exist it silently returns defaults.
	// Run from a temp dir to ensure the file is absent.
	orig, _ := os.Getwd()
	defer os.Chdir(orig) //nolint:errcheck
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load("")
	if err != nil {
		t.Fatalf("Load empty path: %v", err)
	}
	if cfg.Workers != 4 {
		t.Errorf("default workers: got %d", cfg.Workers)
	}
}

func writeTempYAML(t *testing.T, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "gomnibus.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
