package tokenizer

import (
	"bufio"
	"io"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/lwch/logging"
)

type stat struct {
	word Token
	next Token
}

func (t *Tokenizer) loadSequence(readers []io.ReadSeekCloser) ([]*sequence, int) {
	var wg sync.WaitGroup
	var total atomic.Int64
	seqs := make([]*sequence, len(readers))
	wg.Add(len(readers))
	for i, r := range readers {
		go func(i int, r io.ReadCloser) {
			defer wg.Done()
			var cnt int64
			seqs[i], cnt = t.buildSequence(r)
			total.Add(cnt)
		}(i, r)
	}
	wg.Wait()
	return seqs, int(total.Load())
}

func (t *Tokenizer) buildSequence(r io.ReadCloser) (*sequence, int64) {
	defer r.Close()
	var maxLen int
	for token := range t.specialTokens {
		if len([]byte(token)) > maxLen {
			maxLen = len([]byte(token))
		}
	}
	var buf []byte
	trimSpecialTokens := func() []byte {
		str := string(buf)
		for k := range t.specialTokens {
			if strings.HasSuffix(str, k) {
				return buf[:len(buf)-len([]byte(k))]
			}
		}
		return buf
	}
	rd := bufio.NewReader(r)
	seq := newSequence()
	var cnt int64
	for {
		str, err := rd.ReadString('\n')
		for _, ch := range []byte(str) {
			buf = append(buf, ch)
			if len(buf) >= maxLen {
				buf = trimSpecialTokens()
			}
			if len(buf) > 0 {
				cnt++
				seq.Push(buf[0])
				buf = buf[1:]
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			logging.Error("read bytes: %v", err)
			return seq, cnt
		}
	}
	for len(buf) > 0 {
		cnt++
		seq.Push(buf[0])
		buf = buf[1:]
	}
	return seq, cnt
}

func (t *Tokenizer) getTokens(seqs []*sequence, filter FilterFunc) map[Token]int {
	mps := make([]map[Token]int, len(seqs))
	var total int
	parallel(seqs, func(i int, seq *sequence) {
		mp := make(map[Token]int)
		seq.Range(func(tk Token) {
			mp[tk]++
		})
		mps[i] = mp
		total = len(mp)
	})
	tmp := parallelMerge(mps, total)
	ret := make(map[Token]int, len(tmp))
	for k, v := range tmp {
		if filter != nil {
			if !filter(k, v) {
				continue
			}
		}
		ret[k] = v
	}
	return ret
}

func (t *Tokenizer) getStats(seqs []*sequence, filter FilterFunc) (*stat, int) {
	mps := make([]map[stat]int, len(seqs))
	var total int
	parallel(seqs, func(i int, seq *sequence) {
		mp := make(map[stat]int)
		seq.RangeStat(func(word, next Token) {
			if word.Len()+next.Len() > maxSeq {
				return
			}
			mp[stat{word, next}]++
		})
		mps[i] = mp
		total = len(mp)
	})
	pairs := sortMap(parallelMerge(mps, total))
	logging.Info("%d stats found", len(pairs))
	for _, pair := range pairs {
		if filter != nil {
			dup := func(data []byte) []byte {
				ret := make([]byte, len(data))
				copy(ret, data)
				return ret
			}
			word := dup(pair.stat.word[:pair.stat.word.Len()])
			next := dup(pair.stat.next[:pair.stat.next.Len()])
			tk := buildToken(append(word, next...))
			if !filter(tk, pair.freq) {
				continue
			}
		}
		return &pair.stat, pair.freq
	}
	return nil, 0
}

func (t *Tokenizer) merge(seqs []*sequence, st *stat) {
	parallel(seqs, func(_ int, s *sequence) {
		s.Merge(st)
	})
}
