package tokenizer

import (
	"bufio"
	"io"
	"os"
	"runtime"
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
	word string
	next string
}

func (t *Tokenizer) TrainFiles(files []string, size int) (map[string]int, error) {
	var readers []io.Reader
	for _, file := range files {
		fi, err := os.Stat(file)
		if err != nil {
			return nil, err
		}
		groups := fi.Size()/readBlockSize + 1
		for i := int64(0); i < groups; i++ {
			f, err := os.Open(file)
			if err != nil {
				return nil, err
			}
			defer f.Close()
			if i*readBlockSize >= fi.Size() {
				break
			}
			_, err = f.Seek(i*readBlockSize, io.SeekStart)
			if err != nil {
				return nil, err
			}
			readers = append(readers, io.LimitReader(f, readBlockSize))
		}
	}
	return t.TrainReaders(readers, size), nil
}

func (t *Tokenizer) Train(str string, size int) map[string]int {
	r := strings.NewReader(str)
	return t.TrainReaders([]io.Reader{r}, size)
}

func (t *Tokenizer) TrainReaders(readers []io.Reader, size int) map[string]int {
	wds := newWords() // {d e e p: 5, l e a r n i n g: 3, ...}
	var wg sync.WaitGroup
	wg.Add(len(readers))
	var pending atomic.Int64
	pending.Add(int64(len(readers)))
	for i, r := range readers {
		go func(i int, r io.Reader) {
			defer wg.Done()
			cnt := getWords(r, wds)
			pending.Add(-1)
			logging.Info("reader %d done, %d rune readen, %d readers pending", i, cnt, pending.Load())
		}(i, r)
	}
	wg.Wait()
	logging.Info("vocab size: %d", wds.Size())
	tokens := getTokens(wds) // {d: 5, e: 8, p: 5, ...}
	logging.Info("got %d tokens of rune", len(tokens))
	if len(tokens) >= size {
		return tokens
	}
	var i int
	for {
		i++
		stats := getStats(wds) // {{d,e}: 5, {e,p}: 5, ...}
		logging.Info("round %d, stats size: %d", i, len(stats))
		if len(stats) == 0 {
			return tokens
		}
		best := bestStat(stats) // {d,e}
		logging.Info("round %d, found best stat: {%s, %s}", i, best.word, best.next)
		wds = mergeVocab(wds, best) // {de e p: 5, l e a r n i n g: 3, ...}
		logging.Info("round %d, vocab size: %d", i, wds.Size())
		tokens = getTokens(wds) // {de: 5, e: 8, p: 5, ...}
		logging.Info("round %d, got %d tokens", i, len(tokens))
		if len(tokens) >= size {
			return tokens
		}
	}
}

func buildBlock(str string) string {
	str = strings.TrimSpace(str)
	tmp := make([]string, 0, len(str))
	for _, ch := range str {
		tmp = append(tmp, string(ch))
	}
	return strings.Join(tmp, " ")
}

func getWords(r io.Reader, wds *words) int {
	rd := bufio.NewReader(r)
	var tmp string
	var cnt int
	for {
		str, err := rd.ReadString('\n')
		for _, ch := range str {
			cnt++
			switch ch {
			case ' ', ',', '.', '?', '!', '\n': // 英文分词
				if len(tmp) > 0 {
					wds.Put(buildBlock(tmp))
					tmp = ""
				}
			case '，', '。', '？', '！': // 中文分词
				if len(tmp) > 0 {
					wds.Put(buildBlock(tmp))
					tmp = ""
				}
			}
			tmp += string(ch)
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
	block string
	freq  int
}

func parallel(wds *words, fn func(p pair)) {
	var wg sync.WaitGroup
	ch := make(chan pair)
	worker := func() {
		defer wg.Done()
		for p := range ch {
			fn(p)
		}
	}
	wg.Add(runtime.NumCPU())
	for i := 0; i < runtime.NumCPU(); i++ {
		go worker()
	}
	wds.Range(func(block string, freq int) {
		ch <- pair{block, freq}
	})
	close(ch)
	wg.Wait()
}

func getTokens(wds *words) map[string]int {
	ret := make(map[string]int)
	var m sync.Mutex
	parallel(wds, func(p pair) {
		for _, ch := range strings.Split(p.block, " ") {
			m.Lock()
			ret[string(ch)] += p.freq
			m.Unlock()
		}
	})
	return ret
}

func getStats(wds *words) map[vocab]int {
	ret := make(map[vocab]int)
	var m sync.Mutex
	parallel(wds, func(p pair) {
		tks := strings.Split(p.block, " ")
		for i := 0; i < len(tks)-1; i++ {
			m.Lock()
			ret[vocab{word: string(tks[i]), next: tks[i+1]}] += p.freq
			m.Unlock()
		}
	})
	return ret
}

func bestStat(stats map[vocab]int) vocab {
	var ret vocab
	for v, f := range stats {
		if f > stats[ret] {
			ret = v
		}
	}
	return ret
}

func mergeVocab(wds *words, best vocab) *words {
	find := best.word + " " + best.next
	replace := best.word + best.next
	ret := newWords()

	parallel(wds, func(p pair) {
		block := p.block
		for {
			tmp := strings.ReplaceAll(block, find, replace)
			if len(tmp) == len(block) {
				break
			}
			block = tmp
		}
		ret.Set(block, p.freq)
	})
	return ret
}
