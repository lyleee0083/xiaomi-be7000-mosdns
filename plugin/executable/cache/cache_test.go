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

package cache

import (
	"bytes"
	"context"
	"strconv"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/IrineSistiana/mosdns/v5/pkg/query_context"
	"github.com/IrineSistiana/mosdns/v5/plugin/executable/sequence"
	"github.com/miekg/dns"
)

func TestCacheLazyUpdateCoalescesPendingQueries(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NewCache(&Args{}, Opts{})
		defer c.Close()

		release := make(chan struct{})
		qCtx := query_context.NewContext(new(dns.Msg).SetQuestion("example.test.", dns.TypeA))
		var refreshes atomic.Int32
		next := sequence.NewChainWalker([]*sequence.ChainNode{{
			E: sequence.ExecutableFunc(func(_ context.Context, refreshed *query_context.Context) error {
				if refreshed == qCtx || refreshed.Q() == qCtx.Q() {
					t.Error("refresh must use a copy of the query")
				}
				refreshes.Add(1)
				<-release
				return nil
			}),
		}}, nil)

		c.doLazyUpdate("first", qCtx, next)
		synctest.Wait()
		for range 32 {
			go c.doLazyUpdate("first", qCtx, next)
		}
		c.doLazyUpdate("second", qCtx, next)
		synctest.Wait()
		if got := refreshes.Load(); got != 2 {
			t.Errorf("pending refreshes = %d, want 2", got)
		}

		close(release)
		synctest.Wait()
		c.doLazyUpdate("first", qCtx, next)
		synctest.Wait()
		if got := refreshes.Load(); got != 3 {
			t.Errorf("total refreshes = %d, want 3 after the first refresh completes", got)
		}
	})
}

func BenchmarkCacheLazyUpdatePending(b *testing.B) {
	c := NewCache(&Args{}, Opts{})
	defer c.Close()

	started := make(chan struct{})
	release := make(chan struct{})
	finished := make(chan struct{})
	qCtx := query_context.NewContext(new(dns.Msg).SetQuestion("example.test.", dns.TypeA))
	next := sequence.NewChainWalker([]*sequence.ChainNode{{
		E: sequence.ExecutableFunc(func(context.Context, *query_context.Context) error {
			close(started)
			<-release
			close(finished)
			return nil
		}),
	}}, nil)
	c.doLazyUpdate("pending", qCtx, next)
	<-started

	b.ReportAllocs()
	for b.Loop() {
		c.doLazyUpdate("pending", qCtx, next)
	}

	close(release)
	<-finished
}

func Test_cachePlugin_Dump(t *testing.T) {
	c := NewCache(&Args{Size: 16 * dumpBlockSize}, Opts{}) // Big enough to create dump fragments.

	resp := new(dns.Msg)
	resp.SetQuestion("test.", dns.TypeA)

	now := time.Now()
	hourLater := now.Add(time.Hour)
	v := &item{
		resp:           resp,
		storedTime:     now,
		expirationTime: hourLater,
	}

	// Fill the cache
	for i := 0; i < 32*dumpBlockSize; i++ {
		c.backend.Store(key(strconv.Itoa(i)), v, hourLater)
	}

	buf := new(bytes.Buffer)
	enw, err := c.writeDump(buf)
	if err != nil {
		t.Fatal(err)
	}
	enr, err := c.readDump(buf)
	if err != nil {
		t.Fatal(err)
	}

	if enw != enr {
		t.Fatalf("read err, wrote %d entries, read %d", enw, enr)
	}
}
