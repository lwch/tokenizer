package tokenizer

import (
	"runtime"
	"sort"
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

func parallelMerge[Key stat | staticToken](arr []map[Key]int, total int) map[Key]int {
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

func sortMap[Key stat](mp map[Key]int) []Key {
	type pair struct {
		key Key
		val int
	}
	var arr []pair
	for k, v := range mp {
		arr = append(arr, pair{k, v})
	}
	sort.Slice(arr, func(i, j int) bool {
		return arr[i].val > arr[j].val
	})
	ret := make([]Key, len(arr))
	for i, p := range arr {
		ret[i] = p.key
	}
	return ret
}
