package util

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"net"
	"time"

	"github.com/hashicorp/yamux"
	"github.com/juju/ratelimit"
)

var (
	// ErrLimitReached signals limit being reached while reading a stream.
	ErrLimitReached = errors.New("limit reached")
)

const defaultBufferLength = 32 * 1024

// ContextReader returns a reader that respects context deadline.
func ContextReader(ctx context.Context, r io.Reader) io.Reader {
	if deadline, ok := ctx.Deadline(); ok {
		type deadliner interface {
			SetReadDeadline(time.Time) error
		}

		if d, ok := r.(deadliner); ok {
			d.SetReadDeadline(deadline) // nolint: errcheck
		}
	}

	return contextReader{ctx, r}
}

type contextReader struct {
	ctx context.Context
	r   io.Reader
}

func (r contextReader) Read(p []byte) (n int, err error) {
	if err = r.ctx.Err(); err != nil {
		return
	}

	if n, err = r.r.Read(p); err != nil {
		if e := r.ctx.Err(); e != nil {
			err = e
		} else if e, ok := err.(*net.OpError); ok && e.Timeout() {
			err = context.DeadlineExceeded
		} else if errors.Is(err, yamux.ErrTimeout) {
			err = context.DeadlineExceeded
		}

		return
	}

	err = r.ctx.Err()

	return
}

// ReadLimitedUntil reads through reader until it is finished with delim or we surpass our limit.
func ReadLimitedUntil(ctx context.Context, r io.Reader, delim string, limit int) ([]byte, error) {
	delimLen := len(delim)
	r = ContextReader(ctx, r)

	if limit > 0 {
		r = io.LimitReader(r, int64(limit))
	}

	b := new(bytes.Buffer)
	buf := make([]byte, defaultBufferLength)

	for {
		n, err := r.Read(buf)
		if n > 0 {
			b.Write(buf[:n])
		}

		if limit > 0 && b.Len() == limit && err != io.EOF {
			err = ErrLimitReached
		}

		if n >= delimLen && string(buf[n-delimLen:n]) == delim {
			b.Truncate(b.Len() - delimLen)
			return b.Bytes(), err
		}

		if err != nil {
			return b.Bytes(), err
		}
	}
}

// ReadLimited reads through reader with limit and context.
func ReadLimited(ctx context.Context, r io.Reader, limit int) ([]byte, error) {
	r = ContextReader(ctx, r)
	if limit > 0 {
		r = io.LimitReader(r, int64(limit))
	}

	ret, err := ioutil.ReadAll(r)
	if limit > 0 && len(ret) == limit {
		err = ErrLimitReached
	}

	return ret, err
}

var _ io.ReadWriter = (*rateLimitedReadWriter)(nil)

type rateLimitedReadWriter struct {
	rw     io.ReadWriter
	bucket *ratelimit.Bucket
}

func NewRateLimitedReadWriter(rw io.ReadWriter, bucket *ratelimit.Bucket) io.ReadWriter {
	return &rateLimitedReadWriter{
		rw:     rw,
		bucket: bucket,
	}
}

func (r *rateLimitedReadWriter) Read(buf []byte) (int, error) {
	n, err := r.rw.Read(buf)
	if err != nil {
		return n, err
	}

	if !r.bucket.WaitMaxDuration(int64(n), time.Minute) {
		return n, ErrLimitReached
	}

	return n, nil
}

func (r *rateLimitedReadWriter) Write(p []byte) (n int, err error) {
	if !r.bucket.WaitMaxDuration(int64(len(p)), time.Minute) {
		return n, ErrLimitReached
	}

	return r.rw.Write(p)
}
