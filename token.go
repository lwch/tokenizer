package tokenizer

type Token [maxSeq]uint16

func buildToken(data []uint16) Token {
	var t Token
	copy(t[:], data)
	return t
}

func (t Token) Len() int {
	for i := 0; i < maxSeq; i++ {
		if t[i] == 0 {
			return i
		}
	}
	return maxSeq
}

func (t Token) string(dict *dict) string {
	var ret []byte
	for i := 0; i < maxSeq; i++ {
		if t[i] == 0 {
			break
		}
		ret = append(ret, dict.Byte(t[i]))
	}
	return string(ret)
}

func equal(a, b Token) bool {
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func (t *Token) merge(b Token) {
	copy((*t)[t.Len():], b[:])
}
