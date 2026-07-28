package config

import (
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds global gomnibus configuration, analogous to omnibus.rb in Ruby Omnibus.
type Config struct {
	BaseDir            string   `yaml:"base_dir"`
	CacheDir           string   `yaml:"cache_dir"`
	InstallDirPrefix   string   `yaml:"install_dir_prefix"`
	UseGitCaching      bool     `yaml:"use_git_caching"`
	UseS3Caching       bool     `yaml:"use_s3_caching"`
	S3Bucket           string   `yaml:"s3_bucket"`
	S3Region           string   `yaml:"s3_region"`
	S3AccessKey        string   `yaml:"s3_access_key"`
	S3SecretKey        string   `yaml:"s3_secret_key"`
	S3Profile          string   `yaml:"s3_profile"`
	S3IAMRoleARN       string   `yaml:"s3_iam_role_arn"`
	S3Prefix           string   `yaml:"s3_prefix"`
	AppendTimestamp    bool     `yaml:"append_timestamp"`
	Workers            int      `yaml:"workers"`
	SoftwareDirs       []string `yaml:"software_dirs"`
	LogLevel           string   `yaml:"log_level"`
	PublishDirOverride string   `yaml:"publish_dir"`
}

func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		BaseDir:         filepath.Join(home, ".gomnibus", "build"),
		CacheDir:        filepath.Join(home, ".gomnibus", "cache"),
		UseGitCaching:   true,
		AppendTimestamp: false,
		Workers:         4,
		LogLevel:        "info",
		SoftwareDirs:    []string{"config/software"},
	}
}

func Load(path string) (*Config, error) {
	cfg := Default()
	if path == "" {
		path = "gomnibus.yaml"
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, err
	}
	return cfg, yaml.Unmarshal(data, cfg)
}
