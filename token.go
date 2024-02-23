package tokenizer

type token [maxSeq]uint16

func (t token) Len() int {
	for i := 0; i < maxSeq; i++ {
		if t[i] == 0 {
			return i
		}
	}
	return maxSeq
}

func (t token) String(dict *dict) string {
	var ret []rune
	for i := 0; i < maxSeq; i++ {
		if t[i] == 0 {
			break
		}
		ret = append(ret, dict.Rune(t[i]))
	}
	return string(ret)
}

func equal(a, b token) bool {
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (t *token) Merge(b token) {
	copy((*t)[t.Len():], b[:])
}
