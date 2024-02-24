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
	word token
	next token
}

func buildDict(readers []io.ReadSeekCloser) *dict {
	var wg sync.WaitGroup
	wg.Add(len(readers))
	var readen atomic.Uint64
	var pending atomic.Int64
	pending.Add(int64(len(readers)))
	mps := make([]map[rune]struct{}, len(readers))
	for i, r := range readers {
		go func(i int, r io.Reader) {
			defer wg.Done()
			mp := make(map[rune]struct{})
			rd := bufio.NewReader(r)
			var cnt int
			for {
				str, err := rd.ReadString('\n')
				for _, ch := range str {
					cnt++
					mp[ch] = struct{}{}
				}
				if err != nil {
					if err == io.EOF {
						break
					}
					logging.Error("read rune: %v", err)
					return
				}
			}
			mps[i] = mp

			readen.Add(uint64(cnt))
			pending.Add(-1)
			logging.Info("%d rune readen, %d readers pending", readen.Load(), pending.Load())
		}(i, r)
	}
	wg.Wait()
	ret := make(map[rune]struct{})
	for _, mp := range mps {
		for k := range mp {
			ret[k] = struct{}{}
		}
	}
	for _, r := range readers {
		_, err := r.Seek(0, io.SeekStart)
		if err != nil {
			panic(err)
		}
	}
	return newDict(ret)
}

func (t *Tokenizer) loadSequence(readers []io.ReadSeekCloser, dict *dict) ([]*sequence, int) {
	var wg sync.WaitGroup
	var total atomic.Int64
	seqs := make([]*sequence, len(readers))
	wg.Add(len(readers))
	for i, r := range readers {
		go func(i int, r io.ReadCloser) {
			defer wg.Done()
			var cnt int64
			seqs[i], cnt = t.buildSequence(r, dict)
			total.Add(cnt)
		}(i, r)
	}
	wg.Wait()
	return seqs, int(total.Load())
}

func (t *Tokenizer) buildSequence(r io.ReadCloser, dict *dict) (*sequence, int64) {
	defer r.Close()
	var maxLen int
	for token := range t.specialTokens {
		if len([]rune(token)) > maxLen {
			maxLen = len([]rune(token))
		}
	}
	var buf []rune
	trimSpecialTokens := func() []rune {
		str := string(buf)
		for k := range t.specialTokens {
			if strings.HasSuffix(str, k) {
				return buf[:len(buf)-len([]rune(k))]
			}
		}
		return buf
	}
	rd := bufio.NewReader(r)
	seq := newSequence()
	var cnt int64
	for {
		str, err := rd.ReadString('\n')
		for _, ch := range str {
			buf = append(buf, ch)
			if len(buf) >= maxLen {
				buf = trimSpecialTokens()
			}
			if len(buf) > 0 {
				cnt++
				seq.Push(dict.ID(buf[0]))
				buf = buf[1:]
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			logging.Error("read rune: %v", err)
			return seq, cnt
		}
	}
	for len(buf) > 0 {
		cnt++
		seq.Push(dict.ID(buf[0]))
		buf = buf[1:]
	}
	return seq, cnt
}

func (t *Tokenizer) getTokens(seqs []*sequence, dict *dict, filter FilterFunc) map[string]int {
	mps := make([]map[token]int, len(seqs))
	var total int
	parallel(seqs, func(i int, seq *sequence) {
		mp := make(map[token]int)
		seq.Range(func(tk token) {
			mp[tk]++
		})
		mps[i] = mp
		total = len(mp)
	})
	tmp := parallelMerge(mps, total)
	if filter != nil {
		for k, v := range tmp {
			if !filter(k.String(dict), v) {
				delete(tmp, k)
			}
		}
	}
	ret := make(map[string]int, len(tmp))
	for k, v := range tmp {
		ret[k.String(dict)] = v
	}
	return ret
}

func (t *Tokenizer) getStats(seqs []*sequence, maxLength, expect int) []stat {
	mps := make([]map[stat]int, len(seqs))
	var total int
	parallel(seqs, func(i int, seq *sequence) {
		mp := make(map[stat]int)
		seq.RangeStat(func(word, next token) {
			if word.Len()+next.Len() > maxLength {
				return
			}
			mp[stat{word, next}]++
		})
		mps[i] = mp
		total = len(mp)
	})
	stats := sortMap(parallelMerge(mps, total))
	logging.Info("%d stats found", len(stats))
	var ret []stat
	exists := make(map[token]struct{})
	for _, stat := range stats {
		if _, ok := exists[stat.word]; ok {
			continue
		}
		if _, ok := exists[stat.next]; ok {
			continue
		}
		exists[stat.word] = struct{}{}
		exists[stat.next] = struct{}{}
		ret = append(ret, stat)
		if len(ret) >= expect {
			break
		}
	}
	return ret
}

func (t *Tokenizer) merge(seqs []*sequence, stats []stat) {
	parallel(seqs, func(_ int, s *sequence) {
		s.Merge(stats)
	})
}
