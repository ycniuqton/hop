package store

import (
	"context"
	"fmt"
	"io"
	"net/url"
	"strings"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// S3 is the AWS S3 (or S3-compatible endpoint) Storage implementation.
// noCopyObject is set once the provider answers CopyObject with NotImplemented.
type S3 struct {
	client       *s3.Client
	uploader     *manager.Uploader
	downloader   *manager.Downloader
	bucket       string
	errs         errorTranslator
	noCopyObject atomic.Bool
	trace        tracer
}

// NewS3 builds a client from explicit credentials; ambient AWS profiles are never consulted.
func NewS3(ctx context.Context, s Settings) (Storage, error) {
	cfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(s.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(s.AccessKeyID, s.SecretAccessKey, "")),
		awsconfig.WithSharedConfigProfile(""),
	)
	if err != nil {
		return nil, fmt.Errorf("aws config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if s.Endpoint != "" {
			o.BaseEndpoint = aws.String(s.Endpoint)
			o.UsePathStyle = true
			o.ResponseChecksumValidation = aws.ResponseChecksumValidationWhenRequired
		}
	})
	uploader := manager.NewUploader(client, func(u *manager.Uploader) {
		u.PartSize = Limits.MultipartPartSize
		u.Concurrency = Limits.UploadConcurrency
	})
	downloader := manager.NewDownloader(client, func(d *manager.Downloader) {
		d.PartSize = Limits.DownloadPartSize
		d.Concurrency = Limits.DownloadWorkers
	})
	return &S3{
		client: client, uploader: uploader, downloader: downloader, bucket: s.Bucket,
		errs: errorTranslator{bucket: s.Bucket}, trace: tracer{w: s.Trace, bucket: s.Bucket},
	}, nil
}

// Put uploads through the multipart-aware uploader.
func (c *S3) Put(ctx context.Context, key string, r io.Reader, _ int64, opt PutOptions) error {
	c.trace.log(OpPut, key)
	in := &s3.PutObjectInput{Bucket: &c.bucket, Key: &key, Body: r, Metadata: opt.Meta}
	if opt.IfNoneMatch {
		in.IfNoneMatch = aws.String(conditional.AnyETag)
	}
	_, err := c.uploader.Upload(ctx, in)
	return c.errs.translate(OpPut, key, err)
}

// Get streams an object body.
func (c *S3) Get(ctx context.Context, key string) (io.ReadCloser, Object, error) {
	c.trace.log(OpGet, key)
	out, err := c.client.GetObject(ctx, &s3.GetObjectInput{Bucket: &c.bucket, Key: &key})
	if err != nil {
		return nil, Object{}, c.errs.translate(OpGet, key, err)
	}
	info := Object{Key: key, Size: aws.ToInt64(out.ContentLength), Modified: aws.ToTime(out.LastModified), Meta: out.Metadata}
	return out.Body, info, nil
}

// Head reads object info.
func (c *S3) Head(ctx context.Context, key string) (Object, error) {
	c.trace.log(OpHead, key)
	out, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: &c.bucket, Key: &key})
	if err != nil {
		return Object{}, c.errs.translate(OpHead, key, err)
	}
	return Object{Key: key, Size: aws.ToInt64(out.ContentLength), Modified: aws.ToTime(out.LastModified), Meta: out.Metadata}, nil
}

// Delete removes keys in batches of 1000.
func (c *S3) Delete(ctx context.Context, keys ...string) error {
	for start := 0; start < len(keys); start += Limits.DeleteBatch {
		end := min(start+Limits.DeleteBatch, len(keys))
		if err := c.deleteBatch(ctx, keys[start:end]); err != nil {
			return err
		}
	}
	return nil
}

func (c *S3) deleteBatch(ctx context.Context, keys []string) error {
	c.trace.log(OpDelete, fmt.Sprintf("%s (+%d)", keys[0], len(keys)-1))
	ids := make([]types.ObjectIdentifier, len(keys))
	for i := range keys {
		ids[i] = types.ObjectIdentifier{Key: &keys[i]}
	}
	out, err := c.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: &c.bucket, Delete: &types.Delete{Objects: ids, Quiet: aws.Bool(true)},
	})
	if err != nil {
		return c.errs.translate(OpDelete, keys[0], err)
	}
	if len(out.Errors) > 0 {
		e := out.Errors[0]
		return c.errs.fromCode(OpDelete, aws.ToString(e.Key), aws.ToString(e.Code), aws.ToString(e.Message))
	}
	return nil
}

// List pages through ListObjectsV2 until exhausted or limit entries are collected.
func (c *S3) List(ctx context.Context, prefix, delimiter string, limit int) ([]Object, []string, error) {
	c.trace.log(OpList, prefix)
	in := &s3.ListObjectsV2Input{Bucket: &c.bucket, Prefix: &prefix}
	if delimiter != "" {
		in.Delimiter = &delimiter
	}
	lim := listLimit(limit)
	in.MaxKeys = lim.pageSize()
	var objs []Object
	var dirs []string
	pages := s3.NewListObjectsV2Paginator(c.client, in)
	for pages.HasMorePages() && !lim.reached(len(objs)+len(dirs)) {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, nil, c.errs.translate(OpList, prefix, err)
		}
		for _, o := range page.Contents {
			objs = append(objs, Object{Key: aws.ToString(o.Key), Size: aws.ToInt64(o.Size), Modified: aws.ToTime(o.LastModified)})
		}
		for _, p := range page.CommonPrefixes {
			dirs = append(dirs, aws.ToString(p.Prefix))
		}
	}
	objs, dirs = lim.trim(objs, dirs)
	return objs, dirs, nil
}

// copySource percent-encodes every key segment, including '+', which servers may decode as a space.
func (c *S3) copySource(key string) *string {
	segments := strings.Split(key, "/")
	for i, seg := range segments {
		segments[i] = strings.ReplaceAll(url.PathEscape(seg), "+", "%2B")
	}
	return aws.String(url.PathEscape(c.bucket) + "/" + strings.Join(segments, "/"))
}

// conditional holds S3 conditional-request header values.
var conditional = struct{ AnyETag string }{AnyETag: "*"}

// listLimit caps how many entries a List call collects; Limits.NoLimit means unbounded.
type listLimit int

func (l listLimit) reached(n int) bool {
	return int(l) != Limits.NoLimit && n >= int(l)
}

func (l listLimit) pageSize() *int32 {
	if int(l) == Limits.NoLimit || int(l) > Limits.ListPageMax {
		return nil
	}
	return aws.Int32(int32(l))
}

func (l listLimit) trim(objs []Object, dirs []string) ([]Object, []string) {
	if int(l) == Limits.NoLimit {
		return objs, dirs
	}
	dirs = dirs[:min(len(dirs), int(l))]
	objs = objs[:min(len(objs), int(l)-len(dirs))]
	return objs, dirs
}

// Download fetches an object with parallel ranged GETs into w.
func (c *S3) Download(ctx context.Context, key string, w io.WriterAt) (int64, error) {
	c.trace.log(OpGet, key)
	n, err := c.downloader.Download(ctx, w, &s3.GetObjectInput{Bucket: &c.bucket, Key: &key})
	return n, c.errs.translate(OpGet, key, err)
}

// CheckBucket confirms the bucket exists and the keys can reach it.
func (c *S3) CheckBucket(ctx context.Context) error {
	c.trace.log(OpHeadBucket, "")
	_, err := c.client.HeadBucket(ctx, &s3.HeadBucketInput{Bucket: &c.bucket})
	return c.errs.translate(OpHeadBucket, "", err)
}

// CanListAllBuckets reports whether the keys are broader than one bucket.
func (c *S3) CanListAllBuckets(ctx context.Context) bool {
	c.trace.log(OpListBuckets, "")
	_, err := c.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	return err == nil
}
