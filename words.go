package tokenizer

import (
	"strings"
	"sync"
)

const maxSeq = 32 // 单个token最大允许由32个字符组成

type block [maxSeq * maxSeq]rune

func buildBlock(str string) block {
	var ret block
	for i, ch := range str {
		ret[i*maxSeq] = ch
	}
	return ret
}

func (b block) Len() int {
	for i := 0; i < maxSeq; i++ {
		if b[i*maxSeq] == 0 {
			return i
		}
	}
	return maxSeq
}

func (b block) Get(n int) word {
	var wd word
	idx := n * maxSeq
	for i := 0; i < maxSeq; i++ {
		if b[idx] == 0 {
			return wd
		}
		wd[i] = b[idx]
		idx++
	}
	return wd
}

func (b *block) Merge(word, next word) bool {
	n := b.Len()
	for i := 0; i < n-1; i++ {
		if b.Get(i).Equal(word) && b.Get(i+1).HasPrefix(next) {
			b.merge(i, next.Len())
			return true
		}
	}
	return false
}

func (b *block) merge(i, size int) {
	curr := b[i*maxSeq : (i+1)*maxSeq]
	next := b[(i+1)*maxSeq : (i+2)*maxSeq]
	for i := 0; i < maxSeq; i++ {
		if curr[i] == 0 {
			copy(curr[i:i+size], next[:size])
			break
		}
	}
	var wd word
	for j := size; j < maxSeq; j++ {
		wd[j-size] = next[j]
	}
	copy(next, wd[:])
	if next[0] == 0 {
		idx := (i + 1) * maxSeq
		copy(b[idx:(maxSeq-1)*maxSeq], b[idx+maxSeq:])
		idx = (maxSeq - 1) * maxSeq
		for j := 0; j < maxSeq; j++ {
			b[idx] = 0
			idx++
		}
	}
}

func (b block) String() string {
	n := b.Len()
	var tmp []string
	for i := 0; i < n; i++ {
		tmp = append(tmp, b.Get(i).String())
	}
	return strings.Join(tmp, " ")
}

type word [maxSeq]rune

func (wd word) Len() int {
	for i := 0; i < maxSeq; i++ {
		if wd[i] == 0 {
			return i
		}
	}
	return maxSeq
}

func (wd word) Equal(a word) bool {
	n1 := wd.Len()
	n2 := a.Len()
	if n1 != n2 {
		return false
	}
	for i := 0; i < n1; i++ {
		if wd[i] != a[i] {
			return false
		}
	}
	return true
}

func (wd word) HasPrefix(prefix word) bool {
	n1 := wd.Len()
	n2 := prefix.Len()
	if n1 < n2 {
		return false
	}
	for i := 0; i < n2; i++ {
		if wd[i] != prefix[i] {
			return false
		}
	}
	return true
}

func (wd word) String() string {
	var str string
	for i := 0; i < maxSeq; i++ {
		if wd[i] == 0 {
			return str
		}
		str += string(wd[i])
	}
	return str
}

type words struct {
	sync.RWMutex
	data map[block]int
}

func newWords() *words {
	return &words{data: make(map[block]int)}
}

func (w *words) Size() int {
	return len(w.data)
}

func (w *words) Put(b block) {
	w.Lock()
	defer w.Unlock()
	w.data[b]++
}

func (w *words) Set(b block, freq int) {
	w.Lock()
	defer w.Unlock()
	w.data[b] = freq
}

func (w *words) Range(fn func(block, int)) {
	w.RLock()
	defer w.RUnlock()
	for k, v := range w.data {
		fn(k, v)
	}
}
