package tokenizer

import (
	"strings"
)

type sequence struct {
	tokens []uint16
	lens   []byte
	size   int
}

func newSequence() *sequence {
	return &sequence{}
}

func (s *sequence) Push(ch uint16) {
	s.tokens = append(s.tokens, ch)
	s.lens = append(s.lens, 1)
	s.size++
}

func (s *sequence) Range(fn func(token)) {
	idx := 0
	for _, l := range s.lens {
		if l == 0 {
			continue
		}
		fn(buildToken(s.tokens[idx : idx+int(l)]))
		idx += int(l)
	}
}

func (s *sequence) RangeStat(fn func(token, token)) {
	idx1 := 0
	l1 := s.lens[0]
	idx2 := int(l1)
	for i := 1; i < len(s.lens); i++ {
		l2 := s.lens[i]
		if l2 == 0 {
			continue
		}
		fn(buildToken(s.tokens[idx1:idx1+int(l1)]), buildToken(s.tokens[idx2:idx2+int(l2)]))
		idx1 += int(l1)
		l1 = l2
		idx2 += int(l2)
	}
}

func (s *sequence) String(dict *dict) string {
	var ret []string
	s.Range(func(tk token) {
		ret = append(ret, "["+tk.String(dict)+"]")
	})
	return strings.Join(ret, " => ")
}

func (s *sequence) Size() int {
	return s.size
}

func (s *sequence) Merge(st *stat) {
	idx1 := 0
	word := buildToken(s.tokens[:s.lens[0]])
	idx2 := int(s.lens[0])
next:
	for i := 1; i < len(s.lens); i++ {
		l := s.lens[i]
		if l == 0 {
			continue
		}
		next := buildToken(s.tokens[idx2 : idx2+int(l)])
		if equal(st.word, word) && equal(st.next, next) {
			word.Merge(next)
			s.lens[i] = 0
			s.lens[idx1] += l
			idx2 += int(l)
			s.size--
			continue next
		}
		word = next
		idx2 += int(l)
		idx1 = i
	}
}
