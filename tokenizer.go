package tokenizer

import (
	"bufio"
	"io"
	"strings"
	"sync"

	"github.com/lwch/logging"
)

type Tokenizer struct {
}

func New() *Tokenizer {
	return &Tokenizer{}
}

type vocab struct {
	word string
	next string
}

func (t *Tokenizer) Train(reader []io.Reader, size int) map[string]int {
	wds := newWords() // {d e e p: 5, l e a r n i n g: 3, ...}
	var wg sync.WaitGroup
	wg.Add(len(reader))
	for _, r := range reader {
		go func(r io.Reader) {
			defer wg.Done()
			getWords(r, wds)
		}(r)
	}
	wg.Wait()
	logging.Info("vocab size: %d", wds.Size())
	tokens := getTokens(wds) // {d: 5, e: 8, p: 5, ...}
	logging.Info("got %d tokens/rune", len(tokens))
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

func getWords(r io.Reader, wds *words) {
	rd := bufio.NewReader(r)
	var tmp string
	for {
		ch, _, err := rd.ReadRune()
		if err != nil {
			if err == io.EOF {
				break
			}
			logging.Error("read rune: %v", err)
			return
		}
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
	if len(tmp) > 0 {
		wds.Put(buildBlock(tmp))
	}
}

func getTokens(wds *words) map[string]int {
	ret := make(map[string]int)
	wds.Range(func(block string, freq int) {
		for _, ch := range strings.Split(block, " ") {
			ret[string(ch)] += freq
		}
	})
	return ret
}

func getStats(wds *words) map[vocab]int {
	ret := make(map[vocab]int)
	wds.Range(func(block string, freq int) {
		tks := strings.Split(block, " ")
		for i := 0; i < len(tks)-1; i++ {
			ret[vocab{word: string(tks[i]), next: tks[i+1]}] += freq
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
	wds.Range(func(block string, freq int) {
		for {
			tmp := strings.ReplaceAll(block, find, replace)
			if len(tmp) == len(block) {
				break
			}
			block = tmp
		}
		ret.Set(block, freq)
	})
	return ret
}
