package tokenizer

import (
	"strings"
	"sync"
)

const maxSeq = 8 // 单个token最大允许由8个字符组成

type block struct {
	length [maxSeq]uint8
	tokens [maxSeq]rune
}

func buildBlock(str []rune) block {
	var ret block
	for i, ch := range str {
		ret.length[i] = 1
		ret.tokens[i] = ch
	}
	return ret
}

func (b block) Len() int {
	var ret int
	for i := 0; i < maxSeq; i++ {
		ret += int(b.length[i])
	}
	return ret
}

func (b block) Get(n int) word {
	var wd word
	var idx int
	for i := 0; i < n; i++ {
		idx += int(b.length[i])
	}
	copy(wd[:b.length[n]], b.tokens[idx:idx+int(b.length[n])])
	return wd
}

func (b *block) Merge(word, next word, idx int) int {
	n := b.Len()
	for i := idx; i < n-1; i++ {
		if b.Get(i).Equal(word) && b.Get(i+1).HasPrefix(next) {
			b.merge(i, next.Len())
			return i + 1
		}
	}
	return -1
}

func (b *block) merge(i, size int) {
	b.length[i] += uint8(size)
	b.length[i+1] -= uint8(size)
	if b.length[i+1] > 0 {
		return
	}
	for j := i + 1; j < maxSeq-1; j++ {
		b.length[j] = b.length[j+1]
	}
	b.length[maxSeq-1] = 0
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
