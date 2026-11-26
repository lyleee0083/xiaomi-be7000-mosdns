package pool

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/miekg/dns"
)

func TestPackBufferWire(t *testing.T) {
	query := new(dns.Msg).SetQuestion("example.test.", dns.TypeA)
	query.Id = 0x1234
	reply := new(dns.Msg).SetReply(query)
	reply.Compress = true
	for range 80 {
		reply.Answer = append(reply.Answer, &dns.TXT{
			Hdr: dns.RR_Header{Name: "example.test.", Rrtype: dns.TypeTXT, Class: dns.ClassINET},
			Txt: []string{strings.Repeat("x", 200)},
		})
	}

	for _, msg := range []*dns.Msg{query, reply} {
		want, err := msg.Pack()
		if err != nil {
			t.Fatal(err)
		}
		packed, err := PackBuffer(msg)
		if err != nil {
			t.Fatal(err)
		}
		matches := bytes.Equal(*packed, want)
		ReleaseBuf(packed)
		if !matches {
			t.Fatalf("incorrect UDP wire for %d-byte message", len(want))
		}

		framed, err := PackTCPBuffer(msg)
		if err != nil {
			t.Fatal(err)
		}
		size := binary.BigEndian.Uint16(*framed)
		matches = bytes.Equal((*framed)[2:], want)
		ReleaseBuf(framed)
		if int(size) != len(want) || !matches {
			t.Fatalf("incorrect TCP frame for %d-byte message", len(want))
		}
	}
}

func TestPackTCPBufferOversize(t *testing.T) {
	msg := new(dns.Msg).SetQuestion("example.test.", dns.TypeTXT)
	for range 275 {
		msg.Answer = append(msg.Answer, &dns.TXT{
			Hdr: dns.RR_Header{Name: "example.test.", Rrtype: dns.TypeTXT, Class: dns.ClassINET},
			Txt: []string{strings.Repeat("x", 255)},
		})
	}
	if frame, err := PackTCPBuffer(msg); err == nil {
		ReleaseBuf(frame)
		t.Fatal("packed an oversized TCP DNS frame")
	}
}

func BenchmarkPackBuffer(b *testing.B) {
	query := new(dns.Msg).SetQuestion("example.test.", dns.TypeA)
	reply := new(dns.Msg).SetReply(query)
	reply.Compress = true
	for range 4 {
		reply.Answer = append(reply.Answer, &dns.TXT{
			Hdr: dns.RR_Header{Name: "example.test.", Rrtype: dns.TypeTXT, Class: dns.ClassINET},
			Txt: []string{strings.Repeat("x", 250)},
		})
	}
	for _, tc := range []struct {
		name string
		msg  *dns.Msg
		pack func(*dns.Msg) (*[]byte, error)
	}{
		{"query", query, PackBuffer},
		{"tcp-reply", reply, PackTCPBuffer},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				buf, err := tc.pack(tc.msg)
				if err != nil {
					b.Fatal(err)
				}
				ReleaseBuf(buf)
			}
		})
	}
}
