/*
 * Copyright (C) 2020-2022, IrineSistiana
 *
 * This file is part of mosdns.
 *
 * mosdns is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * mosdns is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <https://www.gnu.org/licenses/>.
 */

package pool

import (
	"encoding/binary"
	"fmt"

	"github.com/miekg/dns"
)

// PackBuffer packs a DNS message into a buffer that the caller must release with ReleaseBuf.
func PackBuffer(m *dns.Msg) (*[]byte, error) {
	return packBuffer(m, 0)
}

// PackTCPBuffer packs a DNS message with its two-byte TCP length prefix.
// The caller must release the returned buffer with ReleaseBuf.
func PackTCPBuffer(m *dns.Msg) (*[]byte, error) {
	return packBuffer(m, 2)
}

func packBuffer(m *dns.Msg, prefix int) (*[]byte, error) {
	// miekg/dns v1.1.72 needs the uncompressed length plus a spare byte
	// even for compressed messages. Size a shallow copy without changing m.
	uncompressed := *m
	uncompressed.Compress = false
	buf := GetBuf(prefix + uncompressed.Len() + 1)
	dst := (*buf)[prefix:cap(*buf)]
	wire, err := m.PackBuffer(dst)
	if err != nil {
		ReleaseBuf(buf)
		return nil, err
	}

	if prefix != 0 && len(wire) > dns.MaxMsgSize {
		ReleaseBuf(buf)
		return nil, fmt.Errorf("dns payload size %d is too large", len(wire))
	}

	size := prefix + len(wire)
	if &wire[0] == &dst[0] {
		*buf = (*buf)[:size]
	} else {
		// PackBuffer may allocate its own slice. Keep the pool's capacity
		// invariant by copying only in that case.
		ReleaseBuf(buf)
		buf = GetBuf(size)
		copy((*buf)[prefix:], wire)
	}

	if prefix != 0 {
		binary.BigEndian.PutUint16(*buf, uint16(len(wire)))
	}
	return buf, nil
}
