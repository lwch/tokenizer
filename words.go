package tokenizer

import (
	"strings"
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
	for i := 0; i < maxSeq; i++ {
		if b.length[i] == 0 {
			return i
		}
	}
	return maxSeq
}

func (b block) Get(n int) string {
	var idx int
	for i := 0; i < n; i++ {
		idx += int(b.length[i])
	}
	return string(b.tokens[idx : idx+int(b.length[n])])
}

func (b *block) Merge(word, next string, idx int) int {
	n := b.Len()
	for i := idx; i < n-1; i++ {
		if b.Get(i) == word && b.Get(i+1) == next {
			b.merge(i)
			return i + 1
		}
	}
	return -1
}

func (b *block) merge(i int) {
	b.length[i] += b.length[i+1]
	for j := i + 1; j < maxSeq-1; j++ {
		b.length[j] = b.length[j+1]
	}
	b.length[maxSeq-1] = 0
}

func (b block) String() string {
	n := b.Len()
	var tmp []string
	for i := 0; i < n; i++ {
		tmp = append(tmp, b.Get(i))
	}
	return strings.Join(tmp, " ")
}

type words map[block]int

func newWords(arr []map[block]int) words {
	wds := make(words)
	for _, mp := range arr {
		for k, v := range mp {
			wds[k] += v
		}
	}
	return wds
}

func (wds words) Size() int {
	return len(wds)
}

func (wds words) Range(fn func(block, int)) {
	for k, v := range wds {
		fn(k, v)
	}
}
