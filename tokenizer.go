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

	"github.com/lwch/logging"
)

const readBlockSize = 1_000_000 // 1M

type Tokenizer struct {
}

func New() *Tokenizer {
	return &Tokenizer{}
}

type vocab struct {
	word word
	next word
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

func (t *Tokenizer) TrainFiles(files []string, size int) (<-chan map[string]int, error) {
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
	return t.TrainReaders(readers, size), nil
}

func (t *Tokenizer) Train(str string, size int) <-chan map[string]int {
	r := strings.NewReader(str)
	return t.TrainReaders([]io.ReadCloser{io.NopCloser(r)}, size)
}

func (t *Tokenizer) TrainReaders(readers []io.ReadCloser, size int) <-chan map[string]int {
	ch := make(chan map[string]int)
	go func() {
		defer close(ch)
		wds := newWords() // {d e e p: 5, l e a r n i n g: 3, ...}
		var wg sync.WaitGroup
		wg.Add(len(readers))
		var readen atomic.Uint64
		var pending atomic.Int64
		pending.Add(int64(len(readers)))
		for i, r := range readers {
			go func(i int, r io.ReadCloser) {
				defer wg.Done()
				defer r.Close()
				cnt := getWords(r, wds)
				readen.Add(uint64(cnt))
				pending.Add(-1)
				logging.Info("%d rune readen, %d readers pending", readen.Load(), pending.Load())
			}(i, r)
		}
		wg.Wait()
		logging.Info("vocab size: %d", wds.Size())
		tokens := getTokens(wds) // {d: 5, e: 8, p: 5, ...}
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
			bests := bestStats(stats, size-len(tokens)) // {d,e}, ...
			var logs []string
			for _, best := range bests {
				logs = append(logs, fmt.Sprintf("(%s, %s)", best.word, best.next))
			}
			logging.Info("round %d, found best stats: %s", i, strings.Join(logs, " "))
			wds = mergeVocab(wds, bests) // {de e p: 5, l e a r n i n g: 3, ...}
			logging.Info("round %d, vocab size: %d", i, wds.Size())
			tokens = getTokens(wds) // {de: 5, e: 8, p: 5, ...}
			logging.Info("round %d, got %d tokens", i, len(tokens))
			ch <- tokens
			if len(tokens) >= size {
				return
			}
		}
	}()
	return ch
}

func getWords(r io.Reader, wds *words) int {
	rd := bufio.NewReader(r)
	var tmp []rune
	var cnt int
	for {
		str, err := rd.ReadString('\n')
		for _, ch := range str {
			cnt++
			switch ch {
			case ' ', ',', '.', '?', '!', '<', '>', '[', ']', '{', '}', '(', ')', '+', '-', '*', '/', '=', '^', '&', '%', '$', '#', '`', '\\', '|', '"', '\'', '\r', '\n', // 英文分词
				'，', '。', '？', '！', '《', '》', '【', '】', '「', '」', '『', '』', '…', '·', '“', '”', '‘', '’', '、': // 中文分词
				if len(tmp) == 0 {
					continue
				}
				wds.Put(buildBlock(tmp))
				tmp = tmp[:0]
			default:
				tmp = append(tmp, ch)
			}
			if len(tmp) >= maxSeq {
				wds.Put(buildBlock(tmp))
				tmp = tmp[:0]
			}
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			logging.Error("read rune: %v", err)
			return cnt
		}
	}
	if len(tmp) > 0 {
		wds.Put(buildBlock(tmp))
	}
	return cnt
}

type pair struct {
	block block
	freq  int
}

func parallel(wds *words, fn func(i int, p pair)) {
	var wg sync.WaitGroup
	ch := make(chan pair)
	worker := func(i int) {
		defer wg.Done()
		for p := range ch {
			fn(i, p)
		}
	}
	n := runtime.NumCPU()
	// n := 1
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

func getTokens(wds *words) map[string]int {
	mps := make([]map[string]int, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		mps[i] = make(map[string]int)
	}
	parallel(wds, func(i int, p pair) {
		n := p.block.Len()
		for j := 0; j < n; j++ {
			str := p.block.Get(j).String()
			mps[i][str] += p.freq
		}
	})
	ret := make(map[string]int)
	for _, mp := range mps {
		for k, v := range mp {
			ret[k] += v
		}
	}
	return ret
}

func getStats(wds *words) map[vocab]int {
	mps := make([]map[vocab]int, runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		mps[i] = make(map[vocab]int)
	}
	parallel(wds, func(i int, p pair) {
		n := p.block.Len()
		for j := 0; j < n-1; j++ {
			key := vocab{word: p.block.Get(j), next: p.block.Get(j + 1)}
			mps[i][key] += p.freq
		}
	})
	ret := make(map[vocab]int)
	for _, mp := range mps {
		for k, v := range mp {
			ret[k] += v
		}
	}
	return ret
}

func bestStats(stats map[vocab]int, size int) []vocab {
	type pair struct {
		voc  vocab
		freq int
	}
	arr := make([]pair, 0, len(stats))
	for v, f := range stats {
		arr = append(arr, pair{v, f})
	}
	sort.Slice(arr, func(i, j int) bool {
		return arr[i].freq > arr[j].freq
	})
	var ret []vocab
	prefix := make(map[word]struct{})
	for i := 0; i < size; i++ {
		if _, ok := prefix[arr[i].voc.word]; ok {
			return ret
		}
		ret = append(ret, arr[i].voc)
		prefix[arr[i].voc.word] = struct{}{}
	}
	return ret
}

func mergeVocab(wds *words, bests []vocab) *words {
	ret := newWords()
	parallel(wds, func(_ int, p pair) {
		block := p.block
		for {
			changed := false
			for _, best := range bests {
				for block.Merge(best.word, best.next) {
					changed = true
				}
			}
			if !changed {
				break
			}
		}
		ret.Set(block, p.freq)
	})
	return ret
}
