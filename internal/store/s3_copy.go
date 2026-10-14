package store

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

// Copy is a server-side copy; objects above 5 GB use multipart UploadPartCopy.
func (c *S3) Copy(ctx context.Context, src, dst string) error {
	info, err := c.Head(ctx, src)
	if err != nil {
		return err
	}
	if info.Size <= Limits.CopyMaxSingle {
		return c.copySingle(ctx, src, dst)
	}
	return c.copyMultipart(ctx, src, dst, info)
}

func (c *S3) copySingle(ctx context.Context, src, dst string) error {
	c.trace.log(OpCopy, src+" -> "+dst)
	_, err := c.client.CopyObject(ctx, &s3.CopyObjectInput{Bucket: &c.bucket, Key: &dst, CopySource: c.copySource(src)})
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
