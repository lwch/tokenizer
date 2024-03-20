package tokenizer

type Token [maxSeq]byte

func buildToken(data []byte) Token {
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

func (t Token) Bytes() []byte {
	for i := 0; i < maxSeq; i++ {
		if t[i] == 0 {
			return t[:i]
		}
	}
	return t[:]
}

func equal(a, b Token) bool {
	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func less(a, b Token) bool {
	for i := 0; i < len(a); i++ {
		if a[i] < b[i] {
			return true
		} else if a[i] > b[i] {
			return false
		}
	}
	return false
}

func (t *Token) merge(b Token) {
	copy((*t)[t.Len():], b[:])
}
