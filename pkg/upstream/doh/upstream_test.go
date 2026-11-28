package doh

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"
	"time"

	"github.com/IrineSistiana/mosdns/v5/pkg/dnsutils"
	"github.com/IrineSistiana/mosdns/v5/pkg/pool"
	"github.com/miekg/dns"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) {
	return f(r)
}

func TestExchangeContext(t *testing.T) {
	// Cover all base64 tail lengths and all values of the two flag bits
	// in the same base64 group as the transaction ID.
	for size := 12; size <= 14; size++ {
		for flag := 0; flag < 4; flag++ {
			query := make([]byte, size)
			for i := range query {
				query[i] = byte(i + 1)
			}
			query[2] |= byte(flag << 6)
			original := bytes.Clone(query)
			expected := bytes.Clone(query)
			clear(expected[:2])

			rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
				key := strings.TrimPrefix(req.URL.RawQuery, "dns=")
				got, err := base64.RawURLEncoding.DecodeString(key)
				if req.Method != http.MethodGet || key == req.URL.RawQuery ||
					err != nil || !bytes.Equal(got, expected) {
					return nil, fmt.Errorf("unexpected DoH query %q: %x, %v", req.URL.RawQuery, got, err)
				}
				got[2] |= 0x80
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(got))}, nil
			})
			u, err := NewUpstream("https://dns.example.test/dns-query", rt, nil)
			if err != nil {
				t.Fatal(err)
			}
			resp, err := u.ExchangeContext(t.Context(), query)
			if err != nil {
				t.Fatal(err)
			}
			wantResponse := bytes.Clone(original)
			wantResponse[2] |= 0x80
			matches := bytes.Equal(*resp, wantResponse)
			pool.ReleaseBuf(resp)
			if !matches || !bytes.Equal(query, original) {
				t.Fatalf("size %d, flags %d: response changed incorrectly or query was mutated", size, flag)
			}
		}
	}
}

func TestExchangeContextShortQuery(t *testing.T) {
	u, err := NewUpstream("https://dns.example.test/dns-query", nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := u.ExchangeContext(t.Context(), []byte{1, 2}); !errors.Is(err, dnsutils.ErrPayloadTooSmall) {
		t.Fatalf("expected a short payload error, got %v", err)
	}
}

func TestExchangeContextCallerCancellation(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	cause := errors.New("caller stopped waiting")
	ctx, cancelCause := context.WithCancelCause(ctx)
	defer cancelCause(cause)

	finish := make(chan struct{})
	defer close(finish)
	rt := roundTripFunc(func(req *http.Request) (*http.Response, error) {
		cancelCause(cause)
		select {
		case <-req.Context().Done():
			return nil, req.Context().Err()
		case <-finish:
			return &http.Response{
				StatusCode: http.StatusOK,
				Body:       io.NopCloser(bytes.NewReader(make([]byte, dnsutils.DnsHeaderLen))),
			}, nil
		}
	})
	u, err := NewUpstream("https://dns.example.test/dns-query", rt, nil)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := u.ExchangeContext(ctx, make([]byte, dnsutils.DnsHeaderLen)); !errors.Is(err, cause) {
		t.Fatalf("expected caller cancellation, got %v", err)
	}
}

func TestExchangeContextResponseBody(t *testing.T) {
	for _, tc := range []struct {
		name    string
		size    int
		known   bool
		readErr error
	}{
		{name: "known", size: 128, known: true},
		{name: "unknown", size: 2047},
		{name: "grow", size: 8192},
		{name: "limit", size: dns.MaxMsgSize + 1, known: true},
		{name: "short", size: dnsutils.DnsHeaderLen - 1},
		{name: "read error", readErr: io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			wire := bytes.Repeat([]byte{0xa8}, tc.size)
			rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
				length := int64(-1)
				if tc.known {
					length = int64(tc.size)
				}
				var reader io.Reader = bytes.NewReader(wire)
				if tc.readErr != nil {
					reader = iotest.ErrReader(tc.readErr)
				}
				return &http.Response{
					StatusCode:    http.StatusOK,
					Body:          io.NopCloser(reader),
					ContentLength: length,
				}, nil
			})
			u, err := NewUpstream("https://dns.example.test/dns-query", rt, nil)
			if err != nil {
				t.Fatal(err)
			}
			query := make([]byte, dnsutils.DnsHeaderLen)
			binary.BigEndian.PutUint16(query, 0x1234)
			resp, err := u.ExchangeContext(t.Context(), query)
			if resp != nil {
				defer pool.ReleaseBuf(resp)
			}
			switch {
			case tc.readErr != nil:
				if resp != nil || !errors.Is(err, tc.readErr) {
					t.Fatalf("expected response read error, got %v", err)
				}
			case len(wire) < dnsutils.DnsHeaderLen:
				if resp != nil || !errors.Is(err, dnsutils.ErrPayloadTooSmall) {
					t.Fatalf("expected short response error, got %v", err)
				}
			default:
				if err != nil {
					t.Fatal(err)
				}
				if len(*resp) != min(tc.size, dns.MaxMsgSize) ||
					binary.BigEndian.Uint16(*resp) != 0x1234 ||
					!bytes.Equal((*resp)[2:], wire[2:len(*resp)]) {
					t.Fatal("response bytes changed")
				}
			}
		})
	}
}

func BenchmarkExchangeContext(b *testing.B) {
	for _, size := range []int{64, 1232} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			query := make([]byte, size)
			binary.BigEndian.PutUint16(query, 0x1234)
			response := make([]byte, max(128, size))
			response[2] = 0x80
			rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{
					StatusCode:    http.StatusOK,
					Body:          io.NopCloser(bytes.NewReader(response)),
					ContentLength: int64(len(response)),
				}, nil
			})
			u, err := NewUpstream("https://dns.example.test/dns-query", rt, nil)
			if err != nil {
				b.Fatal(err)
			}
			b.ReportAllocs()
			for b.Loop() {
				resp, err := u.ExchangeContext(context.Background(), query)
				if err != nil {
					b.Fatal(err)
				}
				pool.ReleaseBuf(resp)
			}
		})
	}
}
