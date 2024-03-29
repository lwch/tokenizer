package tokenizer

import (
	"strings"
)

const maxSeq = 16 // 单个token最大允许由16个字符组成

type block struct {
	length [maxSeq]uint8
	tokens token
}

func buildBlock(dict *dict, str []rune) block {
	var ret block
	for i, ch := range str {
		ret.length[i] = 1
		ret.tokens[i] = dict.ID(ch)
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

func (b block) Get(n int) token {
	var idx int
	for i := 0; i < n; i++ {
		idx += int(b.length[i])
	}
	var ret token
	copy(ret[:], b.tokens[idx:idx+int(b.length[n])])
	return ret
}

func (b *block) Merge(word, next token, idx int) int {
	n := b.Len()
	for i := idx; i < n-1; i++ {
		if equal(b.Get(i), word) && equal(b.Get(i+1), next) {
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

func (b block) String(dict *dict) string {
	n := b.Len()
	var tmp []string
	for i := 0; i < n; i++ {
		tmp = append(tmp, b.Get(i).String(dict))
	}
	return strings.Join(tmp, " ")
}

type words []map[block]int

func (wds words) Size() int {
	var ret int
	for _, wd := range wds {
		ret += len(wd)
	}
	return ret
}

func (wds words) Range(fn func(block, int)) {
	for _, wd := range wds {
		for k, v := range wd {
			fn(k, v)
		}
	}
}
