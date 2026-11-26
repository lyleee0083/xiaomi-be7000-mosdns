package fastforward

import (
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/miekg/dns"
)

func TestForwardDoH(t *testing.T) {
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		wire, err := base64.RawURLEncoding.DecodeString(r.URL.Query().Get("dns"))
		query := new(dns.Msg)
		if err != nil || query.Unpack(wire) != nil || query.Id != 0 || r.ProtoMajor != 2 {
			http.Error(w, "unexpected DNS-over-HTTPS request", http.StatusBadRequest)
			return
		}
		reply := new(dns.Msg).SetReply(query)
		resp, err := reply.Pack()
		if err != nil {
			http.Error(w, "failed to pack DNS response", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/dns-message")
		_, _ = w.Write(resp)
	}))
	server.EnableHTTP2 = true
	server.StartTLS()
	defer server.Close()

	for _, concurrent := range []int{1, 3} {
		t.Run(strconv.Itoa(concurrent), func(t *testing.T) {
			f, err := NewForward(&Args{
				Concurrent: concurrent,
				Upstreams: []UpstreamConfig{{
					Addr:               server.URL + "/dns-query",
					InsecureSkipVerify: true,
				}},
			}, Opts{})
			if err != nil {
				t.Fatal(err)
			}
			defer f.Close()

			query := new(dns.Msg).SetQuestion("example.test.", dns.TypeA)
			query.Id = 0x1234
			qCtx := query_context.NewContext(query)
			if err := f.Exec(t.Context(), qCtx); err != nil {
				t.Fatal(err)
			}
			resp := qCtx.R()
			if resp == nil || !resp.Response || resp.Id != query.Id ||
				len(resp.Question) != 1 || resp.Question[0] != query.Question[0] {
				t.Fatalf("unexpected forwarded response: %v", resp)
			}
		})
	}
}
