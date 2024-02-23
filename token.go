package tokenizer

type staticToken [maxSeq]uint16

func (t staticToken) Len() int {
	for i := 0; i < maxSeq; i++ {
		if t[i] == 0 {
			return i
		}
	}
	return maxSeq
}

func (t staticToken) String(dict *dict) string {
	var ret []rune
	for i := 0; i < maxSeq; i++ {
		if t[i] == 0 {
			break
		}
		ret = append(ret, dict.Rune(t[i]))
	}
	return string(ret)
}

type dynamicToken []uint16

func (t dynamicToken) Len() int {
	return len(t)
}

func (t dynamicToken) ToStatic() staticToken {
	var ret staticToken
	copy(ret[:], t)
	return ret
}

func (t dynamicToken) String(dict *dict) string {
	var ret []rune
	for _, id := range t {
		ret = append(ret, dict.Rune(id))
	}
	return string(ret)
}

func equal(a dynamicToken, b staticToken) bool {
	if len(a) != b.Len() {
		return false
	}
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (t *dynamicToken) Merge(b staticToken) {
	*t = append(*t, b[:b.Len()]...)
}
