package tokenizer

import (
	"container/list"
)

type sequence struct {
	data list.List
}

func newSequence() *sequence {
	return &sequence{}
}

func (s *sequence) Push(ch uint16) {
	var tk dynamicToken
	tk = append(tk, ch)
	s.data.PushBack(&tk)
}

func (s *sequence) Range(fn func(*dynamicToken)) {
	for e := s.data.Front(); e != nil; e = e.Next() {
		fn(e.Value.(*dynamicToken))
	}
}

func (s *sequence) RangeStat(fn func(*dynamicToken, *dynamicToken)) {
	begin := s.data.Front()
	if begin.Next() == nil {
		return
	}
	for e := begin.Next(); e != nil; e = e.Next() {
		fn(begin.Value.(*dynamicToken), e.Value.(*dynamicToken))
		begin = e
	}
}

func (s *sequence) String(dict *dict) string {
	var ret string
	for e := s.data.Front(); e != nil; e = e.Next() {
		ret += e.Value.(*dynamicToken).String(dict)
	}
	return ret
}

func (s *sequence) Size() int {
	return s.data.Len()
}

func (s *sequence) Merge(stats []stat) bool {
	begin := s.data.Front()
	if begin == nil {
		return false
	}
	var changed bool
next:
	for e := begin.Next(); e != nil; e = e.Next() {
		for _, stat := range stats {
			if equal(*begin.Value.(*dynamicToken), stat.word) && equal(*e.Value.(*dynamicToken), stat.next) {
				begin.Value.(*dynamicToken).Merge(stat.next)
				s.data.Remove(e)
				changed = true
				continue next
			}
		}
		begin = e
	}
	return changed
}
