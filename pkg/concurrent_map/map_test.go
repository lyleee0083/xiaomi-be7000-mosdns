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

package concurrent_map

import (
	"errors"
	"strconv"
	"sync"
	"testing"
)

type testMapHashable uint64

func (h testMapHashable) Sum() uint64 {
	return uint64(h)
}

func TestMapCacheCapacity(t *testing.T) {
	for _, size := range []int{1, 63, 64, 65, 65535} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			m := NewMapCache[testMapHashable, int](size)
			for i := range size {
				m.Set(testMapHashable(i), i)
			}
			if got := m.Len(); got != size {
				t.Fatalf("filled cache has %d entries, want %d", got, size)
			}

			m.Set(0, -1)
			if got := m.Len(); got != size {
				t.Fatalf("cache has %d entries after updating a key, want %d", got, size)
			}

			for i := size; i < 2*size; i++ {
				if i%2 == 0 {
					m.Set(testMapHashable(i), i)
				} else {
					m.TestAndSet(testMapHashable(i), func(int, bool) (int, bool, bool) {
						return i, true, false
					})
				}
			}
			if got := m.Len(); got != size {
				t.Fatalf("cache has %d entries after further inserts, want %d", got, size)
			}
		})
	}
}

func Test_Map(t *testing.T) {
	m := NewMap[testMapHashable, int]()
	wg := sync.WaitGroup{}

	// test add
	for i := 0; i < 512; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Set(testMapHashable(i), i)
		}()
	}
	wg.Wait()

	// test range
	wantErr := errors.New("")
	f := func(key testMapHashable, v int) (newV int, setV bool, deleteV bool, err error) {
		return 0, false, false, wantErr
	}
	if wantErr != m.RangeDo(f) {
		t.Fatal("range should return a error")
	}

	cc := make([]bool, 512)
	f = func(key testMapHashable, v int) (newV int, setV bool, deleteV bool, err error) {
		cc[key] = true
		return 0, false, false, nil
	}
	_ = m.RangeDo(f)
	for _, ok := range cc {
		if !ok {
			t.Fatal("test or range failed")
		}
	}

	// test get
	for i := 0; i < 512; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			v, ok := m.Get(testMapHashable(i))
			if !ok {
				t.Error()
				return
			}
			if v != i {
				t.Error()
				return
			}
		}()
	}
	wg.Wait()

	// test len
	if m.Len() != 512 {
		t.Fatal()
	}

	// test del
	for i := 0; i < 512; i++ {
		i := i
		wg.Add(1)
		go func() {
			defer wg.Done()
			m.Del(testMapHashable(i))
		}()
	}
	wg.Wait()
	if m.Len() != 0 {
		t.Fatal()
	}
}

func TestConcurrentMap_TestAndSet(t *testing.T) {
	cm := NewMap[testMapHashable, int]()
	wg := sync.WaitGroup{}

	f := func(v int, ok bool) (newV int, setV bool, deleteV bool) {
		return 1, true, false
	}

	for i := 0; i < 512; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cm.TestAndSet(1, f)
		}()
	}
	wg.Wait()

	v, _ := cm.Get(1)
	if v != 1 {
		t.Fatal()
	}

	// test delete
	f = func(v int, ok bool) (newV int, setV bool, deleteV bool) {
		return 1, false, true
	}
	cm.TestAndSet(1, f)
	_, ok := cm.Get(1)
	if ok {
		t.Fatal()
	}
}

func BenchmarkConcurrentMap_Get_And_Set(b *testing.B) {
	keys := make([]testMapHashable, 2048)
	m := NewMap[testMapHashable, int]()
	for i := 0; i < 2048; i++ {
		key := testMapHashable(i)
		keys[i] = key
		m.Set(key, i)
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i++
			key := keys[i%2048]
			m.Set(key, i)
			m.Get(key)
		}
	})
}

func Benchmark_RWMutexMap_Get_And_Set(b *testing.B) {
	keys := make([]int, 2048)
	rwm := new(sync.RWMutex)
	m := make(map[int]int, 2048)
	for i := 0; i < 2048; i++ {
		keys[i] = i
		m[i] = i
	}

	b.ReportAllocs()
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			i++
			key := keys[i%2048]

			rwm.Lock()
			m[key] = i
			rwm.Unlock()

			rwm.RLock()
			_ = m[key]
			rwm.RUnlock()
		}
	})
}
