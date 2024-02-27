package tokenizer

type dict struct {
	ch2id map[byte]int
	id2ch []byte
}

func newDict(chars map[byte]struct{}) *dict {
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

func (d *dict) Byte(id uint16) byte {
	return d.id2ch[id]
}

func (d *dict) Encode(s string) []uint16 {
	var ret []uint16
	for _, ch := range []byte(s) {
		ret = append(ret, d.ID(ch))
	}
	return ret
}
