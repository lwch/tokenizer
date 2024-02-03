package tokenizer

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	"github.com/lwch/logging"
)

const readBlockSize = 100_000_000 // 1M

type Tokenizer struct {
	specialTokens map[rune]bool
}

type FilterFunc func(string, int) bool

func New() *Tokenizer {
	return &Tokenizer{
		specialTokens: make(map[rune]bool),
	}
}

func (t *Tokenizer) AddSpecialTokens(token ...rune) {
	for _, v := range token {
		t.specialTokens[v] = true
	}
}

type stat struct {
	l1    int
	l2    int
	words [maxSeq]rune
}

func (s stat) Word() string {
	return string(s.words[:s.l1])
}

func (s stat) Next() string {
	return string(s.words[s.l1 : s.l1+s.l2])
}

type limitReader struct {
	f *os.File
	r io.Reader
}

func newLimitReader(f *os.File, n int64) *limitReader {
	return &limitReader{
		f: f,
		r: io.LimitReader(f, n),
	}
}

func (r *limitReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func (r *limitReader) Close() error {
	return r.f.Close()
}

func (t *Tokenizer) TrainFiles(files []string, size int, filter FilterFunc) (<-chan map[string]int, error) {
	var readers []io.ReadCloser
	clear := func() {
		for _, r := range readers {
			r.Close()
		}
	}
	for _, file := range files {
		fi, err := os.Stat(file)
		if err != nil {
			return nil, err
		}
		groups := fi.Size()/readBlockSize + 1
		for i := int64(0); i < groups; i++ {
			f, err := os.Open(file)
			if err != nil {
				clear()
				return nil, err
			}
			if i*readBlockSize >= fi.Size() {
				break
			}
			_, err = f.Seek(i*readBlockSize, io.SeekStart)
			if err != nil {
				clear()
				return nil, err
			}
			readers = append(readers, newLimitReader(f, readBlockSize))
		}
	}
	return t.TrainReaders(readers, size, filter), nil
}

func (t *Tokenizer) Train(str string, size int, filter FilterFunc) <-chan map[string]int {
	r := strings.NewReader(str)
	return t.TrainReaders([]io.ReadCloser{io.NopCloser(r)}, size, filter)
}

func (t *Tokenizer) TrainReaders(readers []io.ReadCloser, size int, filter func(string, int) bool) <-chan map[string]int {
	ch := make(chan map[string]int, 1)
	go func() {
		defer close(ch)

		var wg sync.WaitGroup
		wg.Add(len(readers))

		var readen atomic.Uint64
		var pending atomic.Int64
		pending.Add(int64(len(readers)))
		wds := make(words, len(readers)) // {d e e p: 5, l e a r n i n g: 3, ...}
		for i, r := range readers {
			go func(i int, r io.ReadCloser) {
				defer wg.Done()
				defer r.Close()
				var cnt int
				wds[i], cnt = getWords(r, t.specialTokens)
				readen.Add(uint64(cnt))
				pending.Add(-1)
				logging.Info("%d rune readen, %d readers pending", readen.Load(), pending.Load())
			}(i, r)
		}
		wg.Wait()
		logging.Info("vocab size: %d", wds.Size())

		tokens := getTokens(wds, filter) // {d: 5, e: 8, p: 5, ...}
		logging.Info("got %d tokens of rune", len(tokens))
		ch <- tokens
		if len(tokens) >= size {
			return
		}
		var i int
		for {
			i++
			stats := getStats(wds) // {{d,e}: 5, {e,p}: 5, ...}
			logging.Info("round %d, stats size: %d", i, len(stats))
			if len(stats) == 0 {
				return
			}
			expect := size - len(tokens)
			expect = int(float64(expect) * 0.1) // 每轮增加10%
			if expect < 1 {
				expect = 1
			}
			bests := bestStats(stats, expect) // {d,e}, ...
			if len(bests) == 0 {
				return
			}
			var logs []string
			for _, best := range bests {
				logs = append(logs, fmt.Sprintf("(%s, %s)", best.Word(), best.Next()))
			}
			logging.Info("round %d, found best stats: %s", i, strings.Join(logs, " "))
			wds = mergeVocab(wds, bests) // {de e p: 5, l e a r n i n g: 3, ...}
			logging.Info("round %d, vocab size: %d", i, wds.Size())
			tokens = getTokens(wds, filter) // {de: 5, e: 8, p: 5, ...}
			logging.Info("round %d, got %d tokens", i, len(tokens))
			ch <- tokens
			if len(tokens) >= size {
				return
			}
		}
	}()
	return ch
}

func getWords(r io.Reader, specialTokens map[rune]bool) (map[block]int, int) {
	ret := make(map[block]int)
	rd := bufio.NewReader(r)
	var tmp []rune
	var cnt int
	for {
		str, err := rd.ReadString('\n')
		for _, ch := range str {
			cnt++
			switch {
			case ch == '%': // 百分比
				if len(tmp) == 0 {
					ret[buildBlock([]rune{ch})]++
					continue
				}
				isNumber := true
				for _, ch := range tmp {
					if (ch < '0' || ch > '9') && ch != '.' {
						isNumber = false
						break
					}
				}
				if !isNumber {
					ret[buildBlock(tmp)]++
					ret[buildBlock([]rune{ch})]++
					tmp = tmp[:0]
				}
			case ch == '.': // 小数
				if len(tmp) == 0 {
					ret[buildBlock([]rune{ch})]++
					continue
				}
				isNumber := true
				for _, ch := range tmp {
					if ch < '0' || ch > '9' {
						isNumber = false
						break
					}
				}
				if !isNumber {
					ret[buildBlock(tmp)]++
					ret[buildBlock([]rune{ch})]++
					tmp = tmp[:0]
				}
			case unicode.IsPunct(ch) || unicode.IsSpace(ch) || specialTokens[ch]:
				if len(tmp) == 0 {
					ret[buildBlock([]rune{ch})]++
					continue
				}
				ret[buildBlock(tmp)]++
				ret[buildBlock([]rune{ch})]++
				tmp = tmp[:0]
			default:
				tmp = append(tmp, ch)
			}
			if len(tmp) >= maxSeq {
				ret[buildBlock(tmp)]++
				tmp = tmp[:0]
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			logging.Error("read rune: %v", err)
			return ret, cnt
		}
	}
	if len(tmp) > 0 {
		ret[buildBlock(tmp)]++
	}
	return ret, cnt
}

type pair struct {
	block block
	freq  int
}

func parallel(wds words, fn func(i int, ch <-chan pair)) {
	var wg sync.WaitGroup
	ch := make(chan pair, 1000)
	worker := func(i int) {
		defer wg.Done()
		fn(i, ch)
	}
	n := runtime.NumCPU()
	// n = 1
	wg.Add(n)
	for i := 0; i < n; i++ {
		go worker(i)
	}
	wds.Range(func(b block, freq int) {
		ch <- pair{b, freq}
	})
	close(ch)
	wg.Wait()
}

func parallelMerge[Key stat | string](arr []map[Key]int, total int) map[Key]int {
	type pair struct {
		key Key
		val int
	}
	ret := make(map[Key]int, total)
	var wg sync.WaitGroup
	var m sync.Mutex
	worker := func(ch chan pair) {
		for p := range ch {
			m.Lock()
			ret[p.key] += p.val
			m.Unlock()
		}
	}
	ch := make(chan pair, 1000)
	n := runtime.NumCPU()
	// n = 1
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			defer wg.Done()
			worker(ch)
		}()
	}
	for _, mp := range arr {
		for k, v := range mp {
			ch <- pair{k, v}
		}
	}
	close(ch)
	wg.Wait()
	return ret
}

func getTokens(wds words, filter FilterFunc) map[string]int {
	mps := make([]map[string]int, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		mps[i] = make(map[string]int)
	}
	var total int
	parallel(wds, func(i int, ch <-chan pair) {
		mp := mps[i]
		for p := range ch {
			n := p.block.Len()
			for j := 0; j < n; j++ {
				str := p.block.Get(j)
				mp[str] += p.freq
			}
			total = len(mps[i]) // 不是准确的，仅用来预估数据量
		}
	})
	ret := parallelMerge(mps, total)
	if filter != nil {
		for k, v := range ret {
			if !filter(k, v) {
				delete(ret, k)
			}
		}
	}
	return ret
}

func getStats(wds words) map[stat]int {
	mps := make([]map[stat]int, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		mps[i] = make(map[stat]int)
	}
	var total int
	parallel(wds, func(i int, p <-chan pair) {
		mp := mps[i]
		for p := range p {
			n := p.block.Len()
			var word string
			if n > 0 {
				word = p.block.Get(0)
			}
			for j := 0; j < n-1; j++ {
				next := p.block.Get(j + 1)
				key := stat{l1: len([]rune(word)), l2: len([]rune(next))}
				copy(key.words[:], []rune(word))
				copy(key.words[key.l1:], []rune(next))
				mp[key] += p.freq
				word = next
			}
			total = len(mps[i]) // 不是准确的，仅用来预估数据量
		}
	})
	return parallelMerge(mps, total)
}

func bestStats(stats map[stat]int, size int) []stat {
	type pair struct {
		voc  stat
		freq int
	}
	arr := make([]pair, 0, len(stats))
	for v, f := range stats {
		arr = append(arr, pair{v, f})
	}
	sort.Slice(arr, func(i, j int) bool {
		return arr[i].freq > arr[j].freq
	})
	if len(arr) < size {
		size = len(arr)
	}
	var ret []stat
	for i := 0; i < size; i++ {
		ret = append(ret, arr[i].voc)
	}
	return ret
}

func mergeVocab(wds words, bests []stat) words {
	mps := make(words, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		mps[i] = make(map[block]int)
	}
	parallel(wds, func(i int, ch <-chan pair) {
		mp := mps[i]
		for p := range ch {
			block := p.block
			for _, best := range bests {
				idx := block.Merge(best.Word(), best.Next(), 0)
				for idx != -1 {
					idx = block.Merge(best.Word(), best.Next(), idx)
				}
			}
			mp[block] = p.freq
		}
	})
	return mps
}
