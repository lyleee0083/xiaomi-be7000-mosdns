// Package ruleset compiles and loads MosDNS domain and IP rule files.
package ruleset

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/domain"
	"github.com/IrineSistiana/mosdns/v5/pkg/matcher/netlist"
)

const (
	Extension = ".mdrs"
	magic     = "MDRS"
	version   = 1
	domainSet = 1
	ipSet     = 2
)

// Domain matcher IDs are part of the version 1 file format.
var domainTypes = [...]string{domain.MatcherDomain, domain.MatcherFull, domain.MatcherKeyword, domain.MatcherRegexp}

type source struct {
	Version int      `json:"version"`
	Type    string   `json:"type"`
	Rules   []string `json:"rules"`
}

// Compile validates a JSON rule set and returns its versioned binary representation.
func Compile(r io.Reader) ([]byte, error) {
	dec := json.NewDecoder(r)
	dec.DisallowUnknownFields()
	var src source
	if err := dec.Decode(&src); err != nil {
		return nil, fmt.Errorf("decode rule source: %w", err)
	}
	var extra json.RawMessage
	if dec.Decode(&extra) != io.EOF {
		return nil, errors.New("unexpected data after rule source")
	}
	if src.Version != version {
		return nil, fmt.Errorf("unsupported rule source version %d", src.Version)
	}
	if src.Rules == nil {
		return nil, errors.New("rules must be an array")
	}

	data := append([]byte(magic), version)
	var encode func([]byte, string) ([]byte, error)
	switch src.Type {
	case "domain":
		data = append(data, domainSet)
		encode = encodeDomain
	case "ip":
		data = append(data, ipSet)
		encode = encodeIP
	default:
		return nil, fmt.Errorf("unsupported rule type %q", src.Type)
	}
	data = binary.AppendUvarint(data, uint64(len(src.Rules)))
	for i, rule := range src.Rules {
		var err error
		data, err = encode(data, rule)
		if err != nil {
			return nil, fmt.Errorf("rule %d: %w", i+1, err)
		}
	}
	return data, nil
}

func encodeDomain(data []byte, rule string) ([]byte, error) {
	typ, pattern, explicit := strings.Cut(rule, ":")
	if !explicit {
		typ, pattern = domain.MatcherDomain, rule
	}
	if !validPattern(pattern) {
		return nil, errors.New("invalid domain pattern")
	}
	for id, name := range domainTypes {
		if typ != name {
			continue
		}
		if name == domain.MatcherRegexp {
			if _, err := regexp.Compile(pattern); err != nil {
				return nil, err
			}
		}
		data = append(data, byte(id))
		data = binary.AppendUvarint(data, uint64(len(pattern)))
		return append(data, pattern...), nil
	}
	return nil, fmt.Errorf("unknown domain matcher %q", typ)
}

func validPattern(pattern string) bool {
	return strings.TrimSpace(pattern) != "" && utf8.ValidString(pattern) && !strings.ContainsRune(pattern, 0)
}

func encodeIP(data []byte, rule string) ([]byte, error) {
	prefix, err := netlist.ParsePrefix(rule)
	if err != nil {
		return nil, err
	}
	prefix = prefix.Masked()
	addr := prefix.Addr().AsSlice()
	data = append(data, byte(len(addr)), byte(prefix.Bits()))
	return append(data, addr...), nil
}

func readBody(data []byte, wantType byte, minRuleSize uint64) (*bytes.Reader, uint64, error) {
	if len(data) < len(magic)+2 || string(data[:len(magic)]) != magic {
		return nil, 0, errors.New("invalid MosDNS binary rule file header")
	}
	if data[len(magic)] != version {
		return nil, 0, fmt.Errorf("unsupported binary rule version %d", data[len(magic)])
	}
	if data[len(magic)+1] != wantType {
		return nil, 0, errors.New("binary rule type does not match the loader")
	}
	body := bytes.NewReader(data[len(magic)+2:])
	count, err := binary.ReadUvarint(body)
	if err != nil || count > uint64(body.Len())/minRuleSize {
		return nil, 0, errors.New("invalid binary rule count")
	}
	return body, count, nil
}

// LoadDomain adds a compiled domain set to the provided matcher.
func LoadDomain(data []byte, matcher *domain.MixMatcher[struct{}]) error {
	body, count, err := readBody(data, domainSet, 3)
	if err != nil {
		return err
	}
	for i := uint64(0); i < count; i++ {
		kind, err := body.ReadByte()
		if err != nil || int(kind) >= len(domainTypes) {
			return fmt.Errorf("rule %d: invalid domain matcher", i+1)
		}
		length, err := binary.ReadUvarint(body)
		if err != nil || length == 0 || length > uint64(body.Len()) {
			return fmt.Errorf("rule %d: invalid domain pattern length", i+1)
		}
		start := len(data) - body.Len()
		pattern := string(data[start : start+int(length)])
		if !validPattern(pattern) {
			return fmt.Errorf("rule %d: invalid domain pattern", i+1)
		}
		if err := matcher.GetSubMatcher(domainTypes[kind]).Add(pattern, struct{}{}); err != nil {
			return fmt.Errorf("rule %d: %w", i+1, err)
		}
		if _, err := body.Seek(int64(length), io.SeekCurrent); err != nil {
			return err
		}
	}
	return requireEnd(body)
}

// LoadIP adds a compiled IP set to the provided list. The caller sorts the list.
func LoadIP(data []byte, list *netlist.List) error {
	body, count, err := readBody(data, ipSet, 6)
	if err != nil {
		return err
	}
	var addrBytes [16]byte
	for i := uint64(0); i < count; i++ {
		size, err := body.ReadByte()
		if err != nil || (size != 4 && size != 16) {
			return fmt.Errorf("rule %d: invalid IP address size", i+1)
		}
		bits, err := body.ReadByte()
		if err != nil || int(bits) > int(size)*8 {
			return fmt.Errorf("rule %d: invalid IP prefix length", i+1)
		}
		if _, err := io.ReadFull(body, addrBytes[:size]); err != nil {
			return fmt.Errorf("rule %d: %w", i+1, err)
		}
		addr, _ := netip.AddrFromSlice(addrBytes[:size])
		prefix := netip.PrefixFrom(addr, int(bits))
		if prefix != prefix.Masked() {
			return fmt.Errorf("rule %d: noncanonical IP network", i+1)
		}
		list.Append(prefix)
	}
	return requireEnd(body)
}

func requireEnd(body *bytes.Reader) error {
	if body.Len() != 0 {
		return errors.New("unexpected data after binary rules")
	}
	return nil
}
