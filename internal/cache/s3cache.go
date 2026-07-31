package cache

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"go.uber.org/zap"
)

// S3Options configures an S3Cache.
type S3Options struct {
	Bucket     string
	Region     string
	Prefix     string // key prefix, e.g. "gomnibus/cache"
	AccessKey  string
	SecretKey  string
	Profile    string
	IAMRoleARN string
	Log        *zap.Logger
}

// S3Cache stores build artefacts in an S3 bucket as gzipped tarballs.
// Object keys follow the pattern: <Prefix>/<cacheKey>.tar.gz
type S3Cache struct {
	opts   S3Options
	client *s3.Client
}

// NewS3Cache creates an S3Cache, establishing the SDK client immediately so
// credential errors surface at construction time rather than mid-build.
func NewS3Cache(ctx context.Context, opts S3Options) (*S3Cache, error) {
	if opts.Bucket == "" {
		return nil, fmt.Errorf("S3Cache: bucket must be set")
	}
	client, err := newS3Client(ctx, opts)
	if err != nil {
		return nil, fmt.Errorf("S3Cache: %w", err)
	}
	return &S3Cache{opts: opts, client: client}, nil
}

func (c *S3Cache) Has(ctx context.Context, key string) (bool, error) {
	_, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(c.opts.Bucket),
		Key:    aws.String(c.objectKey(key)),
	})
	if err != nil {
		var nf *types.NotFound
		if errors.As(err, &nf) {
			return false, nil
		}
		return false, fmt.Errorf("s3 HeadObject %s: %w", c.objectKey(key), err)
	}
	return true, nil
}

func (c *S3Cache) Restore(ctx context.Context, key, destDir string) error {
	c.opts.Log.Info("s3 cache restore", zap.String("key", key), zap.String("bucket", c.opts.Bucket))

	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(c.opts.Bucket),
		Key:    aws.String(c.objectKey(key)),
	})
	if err != nil {
		return fmt.Errorf("s3 GetObject %s: %w", c.objectKey(key), err)
	}
	defer out.Body.Close()

	// Stream directly into a temp file then untar, avoiding holding the whole
	// archive in memory.
	tmp, err := os.CreateTemp("", "gomnibus-s3-restore-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	defer tmp.Close()

	if _, err := io.Copy(tmp, out.Body); err != nil {
		return fmt.Errorf("streaming from s3: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return untar(tmp.Name(), destDir)
}

func (c *S3Cache) Store(ctx context.Context, key, srcDir string) error {
	c.opts.Log.Info("s3 cache store", zap.String("key", key), zap.String("bucket", c.opts.Bucket))

	// Archive to a temp file so we know the size before uploading (S3 PutObject
	// needs Content-Length for large objects without multipart).
	tmp, err := os.CreateTemp("", "gomnibus-s3-store-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	defer tmp.Close()

	if err := archiveDir(srcDir, tmp.Name()); err != nil {
		return fmt.Errorf("archiving for s3: %w", err)
	}

	info, err := tmp.Stat()
	if err != nil {
		return err
	}

	if _, err := tmp.Seek(0, io.SeekStart); err != nil {
		return err
	}

	data, err := io.ReadAll(tmp)
	if err != nil {
		return err
	}

	_, err = c.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        aws.String(c.opts.Bucket),
		Key:           aws.String(c.objectKey(key)),
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(info.Size()),
		ContentType:   aws.String("application/gzip"),
	})
	if err != nil {
		return fmt.Errorf("s3 PutObject %s: %w", c.objectKey(key), err)
	}
	return nil
}

func (c *S3Cache) objectKey(key string) string {
	if c.opts.Prefix != "" {
		return c.opts.Prefix + "/" + key + ".tar.gz"
	}
	return key + ".tar.gz"
}

// ── AWS client construction ──────────────────────────────────────────────────

func newS3Client(ctx context.Context, opts S3Options) (*s3.Client, error) {
	var cfgOpts []func(*awsconfig.LoadOptions) error

	if opts.Region != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithRegion(opts.Region))
	}
	if opts.Profile != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithSharedConfigProfile(opts.Profile))
	}
	if opts.AccessKey != "" && opts.SecretKey != "" {
		cfgOpts = append(cfgOpts, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider(opts.AccessKey, opts.SecretKey, ""),
		))
	}

	cfg, err := awsconfig.LoadDefaultConfig(ctx, cfgOpts...)
	if err != nil {
		return nil, err
	}

	// IAM role assumption layered on top of base credentials.
	if opts.IAMRoleARN != "" {
		stsClient := sts.NewFromConfig(cfg)
		cfg.Credentials = aws.NewCredentialsCache(
			stscreds.NewAssumeRoleProvider(stsClient, opts.IAMRoleARN),
		)
	}

	return s3.NewFromConfig(cfg), nil
}

// ── ChainCache ───────────────────────────────────────────────────────────────

// ChainCache checks a fast local cache before falling back to a remote (S3)
// cache, and populates both on a remote hit or store.
type ChainCache struct {
	local  Cache
	remote Cache
	log    *zap.Logger
}

// NewChainCache returns a cache that checks local first, then remote.
func NewChainCache(local, remote Cache, log *zap.Logger) *ChainCache {
	return &ChainCache{local: local, remote: remote, log: log}
}

func (c *ChainCache) Has(ctx context.Context, key string) (bool, error) {
	if hit, err := c.local.Has(ctx, key); err != nil || hit {
		return hit, err
	}
	return c.remote.Has(ctx, key)
}

func (c *ChainCache) Restore(ctx context.Context, key, destDir string) error {
	if hit, _ := c.local.Has(ctx, key); hit {
		c.log.Debug("chain cache: local hit", zap.String("key", key))
		return c.local.Restore(ctx, key, destDir)
	}
	c.log.Debug("chain cache: remote hit", zap.String("key", key))
	if err := c.remote.Restore(ctx, key, destDir); err != nil {
		return err
	}
	// Backfill local so future builds skip the remote round-trip.
	if err := c.local.Store(ctx, key, destDir); err != nil {
		c.log.Warn("chain cache: failed to backfill local", zap.String("key", key), zap.Error(err))
	}
	return nil
}

func (c *ChainCache) Store(ctx context.Context, key, srcDir string) error {
	localErr := c.local.Store(ctx, key, srcDir)
	remoteErr := c.remote.Store(ctx, key, srcDir)
	if localErr != nil && remoteErr != nil {
		return fmt.Errorf("local: %v; remote: %v", localErr, remoteErr)
	}
	if remoteErr != nil {
		c.log.Warn("chain cache: remote store failed", zap.String("key", key), zap.Error(remoteErr))
	}
	return localErr
}
