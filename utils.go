package tokenizer

import (
	"runtime"
	"sort"
	"strings"
	"sync"
)

func parallel(seqs []*sequence, fn func(int, *sequence)) {
	var wg sync.WaitGroup
	wg.Add(len(seqs))
	for i, seq := range seqs {
		go func(i int, seq *sequence) {
			defer wg.Done()
			fn(i, seq)
		}(i, seq)
	}
	wg.Wait()
}

func parallelMerge[Key stat | Token](arr []map[Key]int, total int) map[Key]int {
	type pair struct {
		key Key
		val int
	}
	ret := make(map[Key]int, total)
	var wg sync.WaitGroup
	var m sync.Mutex
	worker := func(ch chan pair) {
		for p := range ch {
			m.Lock()
			ret[p.key] += p.val
			m.Unlock()
		}
	}
	ch := make(chan pair, 1000)
	n := runtime.NumCPU()
	// n = 1
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			worker(ch)
		}()
	}
	for _, mp := range arr {
		for k, v := range mp {
			ch <- pair{k, v}
		}
	}
	close(ch)
	wg.Wait()
	return ret
}

type pair struct {
	stat stat
	freq int
}

func sortMap(mp map[stat]int) []pair {
	var ret []pair
	for k, v := range mp {
		ret = append(ret, pair{k, v})
	}
	sort.Slice(ret, func(i, j int) bool {
		return ret[i].freq > ret[j].freq
	})
	return ret
}

// 1字节: 0xxxxxxx
// 2字节: 110xxxxx 10xxxxxx
// 3字节: 1110xxxx 10xxxxxx 10xxxxxx
// 4字节: 11110xxx 10xxxxxx 10xxxxxx 10xxxxxx
func getUTF8(data []byte) (bool, []byte, []byte) {
	if data[0]&0xC0 == 0x80 { // 10xxxxxx 10xxxxxx ...
		for i := 0; i < len(data); i++ {
			if data[i]&0xC0 != 0x80 {
				return false, data[:i], data[i:]
			}
		}
		return false, data, nil
	}
	if data[0]&0x80 == 0 { // 1字节
		return true, data[:1], data[1:]
	}
	if data[0]&0xE0 == 0xC0 { // 2字节
		if len(data) < 2 {
			return false, data, nil
		}
		return true, data[:2], data[2:]
	}
	if data[0]&0xF0 == 0xE0 { // 3字节
		if len(data) < 3 {
			return false, data, nil
		}
		return true, data[:3], data[3:]
	}
	if data[0]&0xF8 == 0xF0 { // 4字节
		if len(data) < 4 {
			return false, data, nil
		}
		return true, data[:4], data[4:]
	}
	panic("unreachable")
}

func fmtBytes(data []byte) string {
	var ret []string
	for len(data) > 0 {
		var ok bool
		var ch []byte
		ok, ch, data = getUTF8(data)
		if ok {
			ret = append(ret, string(ch))
			continue
		}
		str := "0x"
		for _, b := range ch {
			str += string("0123456789abcdef"[b>>4]) + string("0123456789abcdef"[b&0x0F])
		}
		ret = append(ret, str)
	}
	return strings.Join(ret, "")
}
