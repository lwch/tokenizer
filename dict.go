package tokenizer

import "math"

type dict struct {
	ch2id map[rune]int
	id2ch []rune
}

func newDict(runes map[rune]struct{}) *dict {
	// 中文常用字符约4000+，加上英文字符和数字，总数约不会超过65535
	if len(runes) >= math.MaxUint16 {
		panic("too many runes")
	}
	d := &dict{
		ch2id: make(map[rune]int, len(runes)),
		id2ch: make([]rune, len(runes)+1),
	}
	var idx int
	for ch := range runes {
		idx++
		d.id2ch[idx] = ch
		d.ch2id[ch] = idx
	}
	return d
}

func (d *dict) Size() int {
	return len(d.id2ch) - 1
}

func (d *dict) ID(ch rune) uint16 {
	return uint16(d.ch2id[ch])
}

func (d *dict) Rune(id uint16) rune {
	return d.id2ch[id]
}
