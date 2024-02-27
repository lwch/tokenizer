package tokenizer

import (
	"fmt"
	"runtime"
	"sort"
	"sync"
	"unicode"
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

func fmtShow(data []byte) string {
	if len(data) == 1 {
		ch := rune(data[0])
		if unicode.IsLetter(ch) ||
			unicode.IsNumber(ch) ||
			unicode.IsPunct(ch) ||
			unicode.IsSpace(ch) {
			return string(ch)
		}
		return fmt.Sprintf("\\u%x", ch)
	}
	var ret string
	for _, ch := range string(data) {
		if unicode.IsLetter(ch) ||
			unicode.IsNumber(ch) ||
			unicode.IsPunct(ch) ||
			unicode.IsSpace(ch) {
			ret += string(ch)
			continue
		}
		var ok bool
		for _, rt := range unicode.Scripts {
			if rt == unicode.Common {
				continue
			}
			if unicode.Is(rt, ch) {
				ret += string(ch)
				ok = true
				break
			}
		}
		if ok {
			continue
		}
		ret += fmt.Sprintf("\\u%x", ch)
	}
	return ret
}
