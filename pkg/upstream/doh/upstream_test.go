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
	"time"

	"github.com/IrineSistiana/mosdns/v5/pkg/dnsutils"
	"github.com/IrineSistiana/mosdns/v5/pkg/pool"
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

func BenchmarkExchangeContext(b *testing.B) {
	for _, size := range []int{64, 1232} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			query := make([]byte, size)
			binary.BigEndian.PutUint16(query, 0x1234)
			response := make([]byte, 128)
			response[2] = 0x80
			rt := roundTripFunc(func(*http.Request) (*http.Response, error) {
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader(response))}, nil
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
