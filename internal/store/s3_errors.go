package store

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"

	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/ycniuqton/hop/internal/apperr"
)

// Op names an S3 operation in errors and -v logs.
type Op string

const (
	OpPut         Op = "PutObject"
	OpGet         Op = "GetObject"
	OpHead        Op = "HeadObject"
	OpDelete      Op = "DeleteObjects"
	OpCopy        Op = "CopyObject"
	OpList        Op = "ListObjectsV2"
	OpHeadBucket  Op = "HeadBucket"
	OpListBuckets Op = "ListBuckets"
)

// failure is one class of S3 error with its sentinel and exit code.
type failure struct {
	sentinel error
	code     apperr.ExitCode
	message  string
	hint     string
}

var failures = struct{ notFound, conflict, denied failure }{
	notFound: failure{ErrNotFound, apperr.ExitNotFound, "not found: %[1]s", ""},
	conflict: failure{ErrPreconditionFailed, apperr.ExitConflict, "already exists: %[1]s", ""},
	denied:   failure{ErrAccessDenied, apperr.ExitAuth, "S3 access denied on %[2]s %[1]s", "check the IAM policy ('hop init --print-policy')"},
}

var failureByCode = map[string]failure{
	"NoSuchKey": failures.notFound, "NotFound": failures.notFound, "NoSuchBucket": failures.notFound,
	"PreconditionFailed": failures.conflict, "ConditionalRequestConflict": failures.conflict,
	"AccessDenied": failures.denied, "Forbidden": failures.denied, "InvalidAccessKeyId": failures.denied,
	"SignatureDoesNotMatch": failures.denied, "AllAccessDisabled": failures.denied,
}

var failureByStatus = map[int]failure{
	http.StatusNotFound:           failures.notFound,
	http.StatusPreconditionFailed: failures.conflict,
	http.StatusForbidden:          failures.denied,
}

// errorTranslator maps SDK errors to hop errors that carry both a sentinel and an exit code.
type errorTranslator struct {
	bucket string
}

func (t errorTranslator) translate(op Op, key string, err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}
	if f, ok := t.classify(err); ok {
		return t.build(f, op, key, err)
	}
	if t.isNetwork(err) {
		return apperr.New(apperr.ExitNetwork, "network error on %s %s: %v", op, t.where(key), t.rootCause(err)).Wrap(errors.Join(ErrNetwork, err))
	}
	return apperr.New(apperr.ExitGeneral, "S3 %s %s failed: %v", op, t.where(key), err).Wrap(err)
}

func (t errorTranslator) fromCode(op Op, key, code, msg string) error {
	if f, ok := failureByCode[code]; ok {
		return t.build(f, op, key, errors.New(msg))
	}
	return apperr.New(apperr.ExitGeneral, "S3 %s %s failed: %s %s", op, t.where(key), code, msg)
}

func (t errorTranslator) classify(err error) (failure, bool) {
	var api smithy.APIError
	if errors.As(err, &api) {
		if f, ok := failureByCode[api.ErrorCode()]; ok {
			return f, true
		}
	}
	var resp *smithyhttp.ResponseError
	if errors.As(err, &resp) {
		f, ok := failureByStatus[resp.HTTPStatusCode()]
		return f, ok
	}
	return failure{}, false
}

func (t errorTranslator) build(f failure, op Op, key string, cause error) error {
	e := apperr.New(f.code, f.message, t.where(key), op).Wrap(errors.Join(f.sentinel, cause))
	if f.hint != "" {
		e = e.WithHint("%s", f.hint)
	}
	return e
}

func (t errorTranslator) isNetwork(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF)
}

func (t errorTranslator) rootCause(err error) error {
	for u := errors.Unwrap(err); u != nil; u = errors.Unwrap(err) {
		err = u
	}
	return err
}

func (t errorTranslator) where(key string) string {
	return fmt.Sprintf("%s/%s", t.bucket, key)
}

// tracer writes one line per S3 call for -v; it never sees credentials.
type tracer struct {
	w      io.Writer
	bucket string
}

func (t tracer) log(op Op, key string) {
	if t.w != nil {
		fmt.Fprintf(t.w, "s3: %s %s/%s\n", op, t.bucket, key)
	}
}
