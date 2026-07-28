// Package s3 fetches software sources directly from Amazon S3.
//
// Credential resolution order (same as AWS CLI):
//  1. Explicit access_key / secret_key in the software source definition
//  2. Environment variables AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY
//  3. Shared credentials file profile (source.s3_profile or AWS_PROFILE)
//  4. IAM role / instance metadata (EC2, ECS, Lambda)
package s3

import (
	"context"
	"crypto/md5"  //nolint:gosec
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/syntaxroot-cc/gomnibus/internal/fetcher"
	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

func init() {
	fetcher.Register(&S3Fetcher{})
}

// S3Fetcher downloads a single S3 object described by a software source.
type S3Fetcher struct{}

func (f *S3Fetcher) Name() string { return "s3" }

// clientFactory creates an S3 client; replaced in tests to inject a fake server.
var clientFactory = newClient

func (f *S3Fetcher) Fetch(ctx context.Context, src *software.Source, destDir string) error {
	if src.S3Bucket == "" || src.S3Key == "" {
		return fmt.Errorf("s3 source requires both s3_bucket and s3_key")
	}

	client, err := clientFactory(ctx, src)
	if err != nil {
		return fmt.Errorf("s3 client: %w", err)
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return err
	}

	out, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(src.S3Bucket),
		Key:    aws.String(src.S3Key),
	})
	if err != nil {
		return fmt.Errorf("s3 GetObject s3://%s/%s: %w", src.S3Bucket, src.S3Key, err)
	}
	defer out.Body.Close()

	dest := filepath.Join(destDir, filepath.Base(src.S3Key))
	f2, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f2.Close()

	var h hash.Hash
	var expected string
	switch {
	case src.SHA256 != "":
		h, expected = sha256.New(), src.SHA256
	case src.SHA512 != "":
		h, expected = sha512.New(), src.SHA512
	case src.SHA1 != "":
		h, expected = sha1.New(), src.SHA1 //nolint:gosec
	case src.MD5 != "":
		h, expected = md5.New(), src.MD5 //nolint:gosec
	}

	var w io.Writer = f2
	if h != nil {
		w = io.MultiWriter(f2, h)
	}
	if _, err := io.Copy(w, out.Body); err != nil {
		os.Remove(dest)
		return fmt.Errorf("streaming s3://%s/%s: %w", src.S3Bucket, src.S3Key, err)
	}

	if h != nil {
		got := hex.EncodeToString(h.Sum(nil))
		if got != expected {
			os.Remove(dest)
			return fmt.Errorf("checksum mismatch for %s: got %s, want %s", src.S3Key, got, expected)
		}
	}
	return nil
}

// newClient builds an S3 client honouring explicit credentials in the source
// definition, falling back to the standard AWS credential chain.
func newClient(ctx context.Context, src *software.Source) (*s3.Client, error) {
	var opts []func(*awsconfig.LoadOptions) error

	// Explicit static credentials take highest priority.
	if src.S3AccessKey != "" && src.S3SecretKey != "" {
		opts = append(opts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(src.S3AccessKey, src.S3SecretKey, ""),
		))
	}

	// Named profile (falls through to default profile if blank).
	if src.S3Profile != "" {
		opts = append(opts, awsconfig.WithSharedConfigProfile(src.S3Profile))
	}

	if src.S3Region != "" {
		opts = append(opts, awsconfig.WithRegion(src.S3Region))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, err
	}
	return s3.NewFromConfig(cfg), nil
}
