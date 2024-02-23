package tokenizer

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/lwch/logging"
)

const readBlockSize = 100_000_000 // 100M
const maxSeq = 8                  // 单个token最大允许由8个字符组成

type Tokenizer struct {
	specialTokens map[string]bool
}

type FilterFunc func(string, int) bool

func New() *Tokenizer {
	return &Tokenizer{
		specialTokens: make(map[string]bool),
	}
}

func (t *Tokenizer) AddSpecialTokens(token ...string) {
	for _, v := range token {
		t.specialTokens[v] = true
	}
}

type limitReader struct {
	f     *os.File
	r     io.Reader
	begin int64
	size  int64
}

func newLimitReader(f *os.File, begin, size int64) (*limitReader, error) {
	_, err := f.Seek(begin, io.SeekStart)
	if err != nil {
		return nil, err
	}
	return &limitReader{
		f:     f,
		r:     io.LimitReader(f, size),
		begin: begin,
		size:  size,
	}, nil
}

func (r *limitReader) Seek(offset int64, whence int) (int64, error) {
	if whence != io.SeekStart {
		return 0, fmt.Errorf("unsupported whence: %d", whence)
	}
	n, err := r.f.Seek(offset+r.begin, io.SeekStart)
	if err != nil {
		return n, err
	}
	r.r = io.LimitReader(r.f, r.size-offset)
	return n - r.begin, nil
}

func (r *limitReader) Read(p []byte) (int, error) {
	return r.r.Read(p)
}

func (r *limitReader) Close() error {
	return r.f.Close()
}

func (t *Tokenizer) TrainFiles(files []string, size int, filter FilterFunc) (<-chan map[string]int, error) {
	var readers []io.ReadSeekCloser
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
			r, err := newLimitReader(f, i*readBlockSize, readBlockSize)
			if err != nil {
				clear()
				return nil, err
			}
			readers = append(readers, r)
		}
	}
	return t.TrainReaders(readers, size, filter), nil
}

type nopCloser struct {
	io.ReadSeeker
}

func (nopCloser) Close() error {
	return nil
}

func (t *Tokenizer) Train(str string, size int, filter FilterFunc) <-chan map[string]int {
	r := strings.NewReader(str)
	return t.TrainReaders([]io.ReadSeekCloser{nopCloser{r}}, size, filter)
}

func (t *Tokenizer) TrainReaders(readers []io.ReadSeekCloser, size int, filter FilterFunc) <-chan map[string]int {
	ch := make(chan map[string]int, 1)
	go func() {
		defer close(ch)

		dict := buildDict(readers)
		logging.Info("dict size: %d", dict.Size())

		seqs, cnt := t.loadSequence(readers, dict)
		logging.Info("total tokens: %d", cnt)

		tokens := t.getTokens(seqs, dict, filter)
		tokens = t.appendSpecialTokens(tokens)
		logging.Info("got %d tokens", len(tokens))

		ch <- tokens
		if len(tokens) >= size {
			return
		}

		var i int
		for {
			i++
			expect := size - len(tokens)
			if expect > 100 {
				expect = int(float64(expect) * 0.1) // 每轮增加10%
				if expect < 1 {
					expect = 1
				}
			}
			logging.Info("round %d, expect %d tokens", i, expect)

			bests := t.getStats(seqs, expect)
			if len(bests) == 0 {
				return
			}
			var logs []string
			for _, best := range bests {
				logs = append(logs, fmt.Sprintf("(%s, %s)",
					best.word.String(dict), best.next.String(dict),
				))
			}
			logging.Info("round %d, best stats: %s", i, strings.Join(logs, " "))

			t.merge(seqs, bests)
			var total int
			for _, seq := range seqs {
				total += seq.Size()
			}
			logging.Info("round %d, %d tokens left", i, total)

			// for _, seq := range seqs {
			// 	fmt.Println(seq.String(dict))
			// }

			tokens := t.getTokens(seqs, dict, filter)
			tokens = t.appendSpecialTokens(tokens)
			logging.Info("round %d, got %d tokens", i, len(tokens))

			ch <- tokens
			if len(tokens) >= size {
				return
			}
		}
	}()
	return ch
}

func (t *Tokenizer) appendSpecialTokens(tokens map[string]int) map[string]int {
	for k := range t.specialTokens {
		if _, ok := tokens[k]; !ok {
			tokens[k] = 0
		}
	}
	return tokens
}
