package s3

import (
	"context"
	"crypto/md5"  //nolint:gosec
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/syntaxroot-cc/gomnibus/internal/software"
)

// ── Name ─────────────────────────────────────────────────────────────────────

func TestName(t *testing.T) {
	if (&S3Fetcher{}).Name() != "s3" {
		t.Error("Name: want s3")
	}
}

// ── validation (no AWS call) ──────────────────────────────────────────────────

func TestFetch_MissingBucket_Error(t *testing.T) {
	err := (&S3Fetcher{}).Fetch(context.Background(), &software.Source{S3Key: "k"}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "s3_bucket") {
		t.Errorf("expected s3_bucket error, got: %v", err)
	}
}

func TestFetch_MissingKey_Error(t *testing.T) {
	err := (&S3Fetcher{}).Fetch(context.Background(), &software.Source{S3Bucket: "b"}, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "s3_key") {
		t.Errorf("expected s3_key error, got: %v", err)
	}
}

func TestFetch_MissingBothBucketAndKey_Error(t *testing.T) {
	err := (&S3Fetcher{}).Fetch(context.Background(), &software.Source{}, t.TempDir())
	if err == nil {
		t.Error("expected error for missing bucket and key")
	}
}

// ── fake-server helpers ───────────────────────────────────────────────────────

// fakeS3 starts an httptest server that returns statusCode and body for every
// request, and patches clientFactory to use it. Returns a cleanup function.
func fakeS3(t *testing.T, statusCode int, body []byte) (*httptest.Server, func()) {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(statusCode)
		w.Write(body) //nolint:errcheck
	}))

	orig := clientFactory
	clientFactory = func(ctx context.Context, src *software.Source) (*awss3.Client, error) {
		cfg, err := awsconfig.LoadDefaultConfig(ctx,
			awsconfig.WithRegion("us-east-1"),
			awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider("test", "test", "")),
			awsconfig.WithRetryMaxAttempts(1),
		)
		if err != nil {
			return nil, err
		}
		return awss3.NewFromConfig(cfg, func(o *awss3.Options) {
			o.BaseEndpoint = aws.String(srv.URL)
			o.UsePathStyle = true
		}), nil
	}

	return srv, func() {
		srv.Close()
		clientFactory = orig
	}
}

func src(bucket, key string, extra ...func(*software.Source)) *software.Source {
	s := &software.Source{S3Bucket: bucket, S3Key: key}
	for _, fn := range extra {
		fn(s)
	}
	return s
}

// ── download ──────────────────────────────────────────────────────────────────

func TestFetch_DownloadsFileContents(t *testing.T) {
	body := []byte("hello from S3")
	_, cleanup := fakeS3(t, http.StatusOK, body)
	defer cleanup()

	destDir := t.TempDir()
	if err := (&S3Fetcher{}).Fetch(context.Background(), src("my-bucket", "path/to/file.tar.gz"), destDir); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	got, err := os.ReadFile(filepath.Join(destDir, "file.tar.gz"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("content: got %q, want %q", got, body)
	}
}

func TestFetch_FilenameFromKeyBase(t *testing.T) {
	_, cleanup := fakeS3(t, http.StatusOK, []byte("data"))
	defer cleanup()

	destDir := t.TempDir()
	if err := (&S3Fetcher{}).Fetch(context.Background(), src("b", "deep/nested/artifact.zip"), destDir); err != nil {
		t.Fatalf("Fetch: %v", err)
	}

	if _, err := os.Stat(filepath.Join(destDir, "artifact.zip")); err != nil {
		t.Error("expected artifact.zip in dest dir")
	}
}

func TestFetch_CreatesDestDir(t *testing.T) {
	_, cleanup := fakeS3(t, http.StatusOK, []byte("data"))
	defer cleanup()

	destDir := filepath.Join(t.TempDir(), "new", "dest")
	if err := (&S3Fetcher{}).Fetch(context.Background(), src("b", "k"), destDir); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, err := os.Stat(destDir); err != nil {
		t.Error("dest dir should be created")
	}
}

func TestFetch_KeyWithoutPath_UsesKeyAsFilename(t *testing.T) {
	_, cleanup := fakeS3(t, http.StatusOK, []byte("data"))
	defer cleanup()

	destDir := t.TempDir()
	if err := (&S3Fetcher{}).Fetch(context.Background(), src("b", "flatkey.tar.gz"), destDir); err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if _, err := os.Stat(filepath.Join(destDir, "flatkey.tar.gz")); err != nil {
		t.Error("expected flatkey.tar.gz in dest dir")
	}
}

// ── checksum happy paths ──────────────────────────────────────────────────────

func TestFetch_NoChecksum_Succeeds(t *testing.T) {
	_, cleanup := fakeS3(t, http.StatusOK, []byte("any content"))
	defer cleanup()

	if err := (&S3Fetcher{}).Fetch(context.Background(), src("b", "k"), t.TempDir()); err != nil {
		t.Fatalf("no-checksum fetch: %v", err)
	}
}

func TestFetch_SHA256_Match(t *testing.T) {
	body := []byte("sha256 content")
	h := sha256.Sum256(body)
	_, cleanup := fakeS3(t, http.StatusOK, body)
	defer cleanup()

	s := src("b", "k", func(s *software.Source) { s.SHA256 = hex.EncodeToString(h[:]) })
	if err := (&S3Fetcher{}).Fetch(context.Background(), s, t.TempDir()); err != nil {
		t.Fatalf("SHA256 match: %v", err)
	}
}

func TestFetch_SHA512_Match(t *testing.T) {
	body := []byte("sha512 content")
	h := sha512.Sum512(body)
	_, cleanup := fakeS3(t, http.StatusOK, body)
	defer cleanup()

	s := src("b", "k", func(s *software.Source) { s.SHA512 = hex.EncodeToString(h[:]) })
	if err := (&S3Fetcher{}).Fetch(context.Background(), s, t.TempDir()); err != nil {
		t.Fatalf("SHA512 match: %v", err)
	}
}

func TestFetch_SHA1_Match(t *testing.T) {
	body := []byte("sha1 content")
	h := sha1.Sum(body) //nolint:gosec
	_, cleanup := fakeS3(t, http.StatusOK, body)
	defer cleanup()

	s := src("b", "k", func(s *software.Source) { s.SHA1 = hex.EncodeToString(h[:]) })
	if err := (&S3Fetcher{}).Fetch(context.Background(), s, t.TempDir()); err != nil {
		t.Fatalf("SHA1 match: %v", err)
	}
}

func TestFetch_MD5_Match(t *testing.T) {
	body := []byte("md5 content")
	h := md5.Sum(body) //nolint:gosec
	_, cleanup := fakeS3(t, http.StatusOK, body)
	defer cleanup()

	s := src("b", "k", func(s *software.Source) { s.MD5 = hex.EncodeToString(h[:]) })
	if err := (&S3Fetcher{}).Fetch(context.Background(), s, t.TempDir()); err != nil {
		t.Fatalf("MD5 match: %v", err)
	}
}

// ── checksum mismatch ─────────────────────────────────────────────────────────

func TestFetch_SHA256_Mismatch_ReturnsError(t *testing.T) {
	_, cleanup := fakeS3(t, http.StatusOK, []byte("content"))
	defer cleanup()

	s := src("b", "k", func(s *software.Source) { s.SHA256 = "badhash" })
	err := (&S3Fetcher{}).Fetch(context.Background(), s, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected checksum mismatch error, got: %v", err)
	}
}

func TestFetch_SHA256_Mismatch_RemovesPartialFile(t *testing.T) {
	_, cleanup := fakeS3(t, http.StatusOK, []byte("content"))
	defer cleanup()

	destDir := t.TempDir()
	s := src("b", "k", func(s *software.Source) { s.SHA256 = "badhash" })
	(&S3Fetcher{}).Fetch(context.Background(), s, destDir) //nolint:errcheck

	entries, _ := os.ReadDir(destDir)
	if len(entries) != 0 {
		t.Errorf("partial file should be removed after checksum mismatch; found %d entries", len(entries))
	}
}

func TestFetch_MD5_Mismatch_ReturnsError(t *testing.T) {
	_, cleanup := fakeS3(t, http.StatusOK, []byte("content"))
	defer cleanup()

	s := src("b", "k", func(s *software.Source) { s.MD5 = "deadbeef" })
	err := (&S3Fetcher{}).Fetch(context.Background(), s, t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("expected checksum mismatch error, got: %v", err)
	}
}

// SHA256 takes priority over lower-precedence hashes when both are set.
func TestFetch_SHA256_Priority_Over_MD5(t *testing.T) {
	body := []byte("priority test")
	sha := sha256.Sum256(body)
	_, cleanup := fakeS3(t, http.StatusOK, body)
	defer cleanup()

	s := src("b", "k", func(s *software.Source) {
		s.SHA256 = hex.EncodeToString(sha[:])
		s.MD5 = "wrongmd5" // would fail if used
	})
	if err := (&S3Fetcher{}).Fetch(context.Background(), s, t.TempDir()); err != nil {
		t.Fatalf("SHA256 should win over MD5: %v", err)
	}
}

// ── server errors ─────────────────────────────────────────────────────────────

func TestFetch_ServerError_ReturnsError(t *testing.T) {
	_, cleanup := fakeS3(t, http.StatusInternalServerError, nil)
	defer cleanup()

	err := (&S3Fetcher{}).Fetch(context.Background(), src("b", "k"), t.TempDir())
	if err == nil {
		t.Error("expected error for server 500")
	}
}

func TestFetch_NotFound_ReturnsError(t *testing.T) {
	_, cleanup := fakeS3(t, http.StatusNotFound, []byte(`<?xml version="1.0"?><Error><Code>NoSuchKey</Code><Message>Key not found</Message><RequestId>1</RequestId><HostId>h</HostId></Error>`))
	defer cleanup()

	err := (&S3Fetcher{}).Fetch(context.Background(), src("b", "missing-key.tar.gz"), t.TempDir())
	if err == nil {
		t.Error("expected error for 404 response")
	}
	if !strings.Contains(err.Error(), "s3 GetObject") {
		t.Errorf("error should mention s3 GetObject, got: %v", err)
	}
}
