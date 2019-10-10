package util

import (
	"context"
	"hash/fnv"
	"io"
	"math/rand"
	"net"
	"strconv"
	"time"
	"unicode/utf8"

	"google.golang.org/grpc/peer"
)

const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// GenerateRandomString creates random string with a-zA-Z0-9 of specified length.
func GenerateRandomString(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}

	return string(b)
}

// GenerateKey creates random string with length=32.
func GenerateKey() string {
	return GenerateRandomString(32)
}

// Retry retries f() and sleeps between retries.
func Retry(attempts int, sleep time.Duration, f func() error) (err error) {
	for i := 0; ; i++ {
		err = f()
		if err == nil {
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)
	}

	return
}

// RetryWithCritical is similar to Retry but allows func to return true/false to mark if returned error is critical or not.
func RetryWithCritical(attempts int, sleep time.Duration, f func() (bool, error)) (err error) {
	for i := 0; ; i++ {
		var critical bool

		critical, err = f()
		if err == nil || critical {
			return
		}

		if i >= (attempts - 1) {
			break
		}

		time.Sleep(sleep)
	}

	return err
}

// Must is a small helper that checks if err is nil. If it's not - panics.
// This should be used only for errors that can never actually happen and are impossible to simulate in tests.
func Must(err error) {
	if err != nil {
		panic(err)
	}
}

// Hash returns FNV 1a hash of string.
func Hash(s string) uint32 {
	h := fnv.New32a()
	_, e := h.Write([]byte(s))

	Must(e)

	return h.Sum32()
}

// PeerAddr returns Address of Peer from context or empty TCPAddr otherwise.
func PeerAddr(ctx context.Context) net.Addr {
	p, _ := peer.FromContext(ctx)
	if p != nil {
		return p.Addr
	}

	return &net.TCPAddr{}
}

// ChannelReader is an object implementing io.Reader where all data is read from the channel.
type ChannelReader struct {
	Channel chan []byte
	buffer  []byte
}

func (cr *ChannelReader) Read(p []byte) (n int, err error) {
	n = copy(p, cr.buffer)
	cr.buffer = cr.buffer[n:]

	for {
		if len(cr.buffer) == 0 {
			cr.buffer = <-cr.Channel
		}

		if cr.buffer != nil {
			copied := copy(p[n:], cr.buffer)
			cr.buffer = cr.buffer[copied:]
			n += copied

			if copied <= len(cr.buffer) {
				break
			}
		} else {
			break
		}
	}

	if n == 0 {
		err = io.EOF
	}

	return
}

const lowerhex = "0123456789abcdef"

// ToQuoteJSON converts source []byte to ASCII only quoted []byte.
func ToQuoteJSON(s []byte) []byte { // nolint: gocyclo
	var width int

	buf := make([]byte, 0, 3*len(s)/2)
	buf = append(buf, '"')

	for ; len(s) > 0; s = s[width:] {
		r := rune(s[0])
		width = 1

		if r == '"' || r == '\\' {
			buf = append(buf, '\\')
			buf = append(buf, byte(r))

			continue
		}

		if r < utf8.RuneSelf && strconv.IsPrint(r) {
			buf = append(buf, byte(r))
			continue
		}

		r, width = utf8.DecodeRune(s)
		switch r {
		case '\b':
			buf = append(buf, `\b`...)
		case '\f':
			buf = append(buf, `\f`...)
		case '\n':
			buf = append(buf, `\n`...)
		case '\r':
			buf = append(buf, `\r`...)
		case '\t':
			buf = append(buf, `\t`...)
		default:
			buf = append(buf, `\u`...)

			switch {
			case r < 0x10000:
				for s := 12; s >= 0; s -= 4 {
					buf = append(buf, lowerhex[r>>uint(s)&0xF])
				}
			default:
				r -= 0x10000
				r = (r >> 10) + 0xd800

				for s := 12; s >= 0; s -= 4 {
					buf = append(buf, lowerhex[r>>uint(s)&0xF])
				}

				buf = append(buf, `\u`...)
				r = (r & 0x3ff) + 0xdc00

				for s := 12; s >= 0; s -= 4 {
					buf = append(buf, lowerhex[r>>uint(s)&0xF])
				}
			}
		}
	}

	buf = append(buf, '"')

	return buf
}
