package tokenizer

import "math"

type dict struct {
	ch2id map[byte]int
	id2ch []byte
}

func newDict(chars map[byte]struct{}) *dict {
	// 中文常用字符约4000+，加上英文字符和数字，总数约不会超过65535
	if len(chars) >= math.MaxUint16 {
		panic("too many bytes")
	}
	d := &dict{
		ch2id: make(map[byte]int, len(chars)),
		id2ch: make([]byte, len(chars)+1),
	}
	var idx int
	for ch := range chars {
		idx++
		d.id2ch[idx] = ch
		d.ch2id[ch] = idx
	}
	return d
}

func (d *dict) Size() int {
	return len(d.id2ch) - 1
}

func (d *dict) ID(ch byte) uint16 {
	return uint16(d.ch2id[ch])
}

func (d *dict) Rune(id uint16) byte {
	return d.id2ch[id]
}
