package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Copy is a server-side copy. Objects above 5 GB use multipart UploadPartCopy, and so does every
// object on providers without CopyObject (Bizfly answers 501); no bytes pass through the client.
func (c *S3) Copy(ctx context.Context, src, dst string) error {
	info, err := c.Head(ctx, src)
	if err != nil {
		return err
	}
	if info.Size <= Limits.CopyMaxSingle && !c.noCopyObject.Load() {
		err := c.copySingle(ctx, src, dst)
		if !errors.Is(err, ErrNotSupported) {
			return err
		}
		c.noCopyObject.Store(true)
	}
	if info.Size == 0 {
		return c.copyEmpty(ctx, dst, info)
	}
	return c.copyMultipart(ctx, src, dst, info)
}

func (c *S3) copySingle(ctx context.Context, src, dst string) error {
	c.trace.log(OpCopy, src+" -> "+dst)
	_, err := c.client.CopyObject(ctx, &s3.CopyObjectInput{Bucket: &c.bucket, Key: &dst, CopySource: c.copySource(src)})
	return c.errs.translate(OpCopy, dst, err)
}

// copyEmpty recreates an empty object, which multipart copy cannot express (it needs one byte).
func (c *S3) copyEmpty(ctx context.Context, dst string, info Object) error {
	c.trace.log(OpCopy, dst+" (empty object rewrite)")
	_, err := c.client.PutObject(ctx, &s3.PutObjectInput{Bucket: &c.bucket, Key: &dst, Body: bytes.NewReader(nil), Metadata: info.Meta})
	return c.errs.translate(OpCopy, dst, err)
}

func (c *S3) copyMultipart(ctx context.Context, src, dst string, info Object) error {
	c.trace.log(OpCopy, fmt.Sprintf("%s -> %s (multipart, %d bytes)", src, dst, info.Size))
	created, err := c.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket: &c.bucket, Key: &dst, Metadata: info.Meta,
	})
	if err != nil {
		return c.errs.translate(OpCopy, dst, err)
	}
	parts, err := c.copyParts(ctx, src, dst, created.UploadId, info.Size)
	if err != nil {
		c.abort(dst, created.UploadId)
		return err
	}
	_, err = c.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: &c.bucket, Key: &dst, UploadId: created.UploadId,
		MultipartUpload: &types.CompletedMultipartUpload{Parts: parts},
	})
	if err != nil {
		c.abort(dst, created.UploadId)
	}
	return c.errs.translate(OpCopy, dst, err)
}

func (c *S3) copyParts(ctx context.Context, src, dst string, uploadID *string, size int64) ([]types.CompletedPart, error) {
	var parts []types.CompletedPart
	for n, off := int32(1), int64(0); off < size; n, off = n+1, off+Limits.CopyPartSize {
		end := min(off+Limits.CopyPartSize, size) - 1
		out, err := c.client.UploadPartCopy(ctx, &s3.UploadPartCopyInput{
			Bucket: &c.bucket, Key: &dst, UploadId: uploadID, PartNumber: aws.Int32(n),
			CopySource: c.copySource(src), CopySourceRange: aws.String(fmt.Sprintf("bytes=%d-%d", off, end)),
		})
		if err != nil {
			return nil, c.errs.translate(OpCopy, dst, err)
		}
		parts = append(parts, types.CompletedPart{ETag: out.CopyPartResult.ETag, PartNumber: aws.Int32(n)})
	}
	return parts, nil
}

func (c *S3) abort(key string, uploadID *string) {
	_, _ = c.client.AbortMultipartUpload(context.Background(), &s3.AbortMultipartUploadInput{
		Bucket: &c.bucket, Key: &key, UploadId: uploadID,
	})
}
