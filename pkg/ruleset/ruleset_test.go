package ruleset

import (
	"bytes"
	"net/netip"
	"strings"
	"testing"

	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/domain"
	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/netlist"
)

func compileTestRules(t *testing.T, source string) []byte {
	t.Helper()
	data, err := Compile(strings.NewReader(source))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestBinaryDomainMatching(t *testing.T) {
	source := `{"version":1,"type":"domain","rules":["Suffix.Example.","full:exact.example","keyword:needle","regexp:^rx[0-9]+\\.example$"]}`
	data := compileTestRules(t, source)
	if !bytes.Equal(data, compileTestRules(t, source)) {
		t.Fatal("compilation is not deterministic")
	}
	matcher := domain.NewDomainMixMatcher()
	if err := LoadDomain(data, matcher); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]bool{
		"sub.suffix.example": true, "EXACT.example.": true,
		"prefix-needle.example": true, "rx42.example": true,
		"sub.exact.example": false, "other.example": false,
	} {
		if _, matched := matcher.Match(name); matched != want {
			t.Errorf("%s matched = %t, want %t", name, matched, want)
		}
	}
}

func TestBinaryIPMatching(t *testing.T) {
	data := compileTestRules(t, `{"version":1,"type":"ip","rules":["192.0.2.1/24","198.51.100.5","2001:db8::2","2001:db8:1::1/48"]}`)
	list := netlist.NewList()
	if err := LoadIP(data, list); err != nil {
		t.Fatal(err)
	}
	list.Sort()
	for addr, want := range map[string]bool{
		"192.0.2.99": true, "198.51.100.5": true, "198.51.100.6": false,
		"2001:db8::2": true, "2001:db8::3": false, "2001:db8:1::123": true,
	} {
		if matched := list.Match(netip.MustParseAddr(addr)); matched != want {
			t.Errorf("%s matched = %t, want %t", addr, matched, want)
		}
	}
}

func TestRejectInvalidRuleFiles(t *testing.T) {
	for _, source := range []string{
		`{"version":2,"type":"ip","rules":[]}`,
		`{"version":1,"type":"ip","rules":["not-an-ip"]}`,
		`{"version":1,"type":"domain","rules":["regexp:["]}`,
		`{"version":1,"type":"domain","rules":["keyword:"]}`,
		`{"version":1,"type":"domain","rules":null}`,
		`{"version":1,"type":"ip","rules":[]} {}`,
	} {
		if _, err := Compile(strings.NewReader(source)); err == nil {
			t.Errorf("accepted invalid source %s", source)
		}
	}
	data := compileTestRules(t, `{"version":1,"type":"domain","rules":["example.test"]}`)
	if err := LoadIP(data, netlist.NewList()); err == nil {
		t.Fatal("IP loader accepted a domain set")
	}
	for _, input := range [][]byte{
		[]byte("example.test\n"), data[:len(data)-1],
		append(bytes.Clone(data), 0),
	} {
		if err := LoadDomain(input, domain.NewDomainMixMatcher()); err == nil {
			t.Error("domain loader accepted invalid binary data")
		}
	}
}
