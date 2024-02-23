package tokenizer

import (
	"container/list"
	"strings"
)

type sequence struct {
	data list.List
}

func newSequence() *sequence {
	return &sequence{}
}

func (s *sequence) Push(ch uint16) {
	var tk token
	tk[0] = ch
	s.data.PushBack(&tk)
}

func (s *sequence) Range(fn func(token)) {
	for e := s.data.Front(); e != nil; e = e.Next() {
		fn(*e.Value.(*token))
	}
}

func (s *sequence) RangeStat(fn func(token, token)) {
	begin := s.data.Front()
	if begin.Next() == nil {
		return
	}
	for e := begin.Next(); e != nil; e = e.Next() {
		fn(*begin.Value.(*token), *e.Value.(*token))
		begin = e
	}
}

func (s *sequence) String(dict *dict) string {
	var ret []string
	for e := s.data.Front(); e != nil; e = e.Next() {
		ret = append(ret, "["+e.Value.(*token).String(dict)+"]")
	}
	return strings.Join(ret, " => ")
}

func (s *sequence) Size() int {
	return s.data.Len()
}

func (s *sequence) Merge(stats []stat) {
	begin := s.data.Front()
	if begin == nil {
		return
	}
next:
	for e := begin.Next(); e != nil; e = e.Next() {
		for _, stat := range stats {
			if equal(*begin.Value.(*token), stat.word) && equal(*e.Value.(*token), stat.next) {
				begin.Value.(*token).Merge(stat.next)
				s.data.Remove(e)
				e = begin
				continue next
			}
		}
		begin = e
	}
}
