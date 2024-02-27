package tokenizer

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/lwch/logging"
)

const maxSeq = 32 // 单个token最大允许由32字节组成

type Tokenizer struct {
	specialTokens map[string]bool
	dict          *dict
}

type FilterFunc func(Token, int) bool

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

func (t *Tokenizer) TrainFiles(files []string, size int, filter FilterFunc) (<-chan map[Token]int, error) {
	var readers []io.ReadSeekCloser
	clear := func() {
		for _, r := range readers {
			r.Close()
		}
	}
	var total int64
	for _, file := range files {
		fi, err := os.Stat(file)
		if err != nil {
			return nil, err
		}
		total += fi.Size()
	}
	blockSize := total / int64(runtime.NumCPU())
	if blockSize < 1_000_000 { // 1M
		blockSize = 1_000_000
	}
	if blockSize == 0 {
		blockSize = total
	}
	for _, file := range files {
		fi, err := os.Stat(file)
		if err != nil {
			return nil, err
		}
		groups := fi.Size()/blockSize + 1
		for i := int64(0); i < groups; i++ {
			f, err := os.Open(file)
			if err != nil {
				clear()
				return nil, err
			}
			if i*blockSize >= fi.Size() {
				break
			}
			r, err := newLimitReader(f, i*blockSize, blockSize)
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

func (t *Tokenizer) Train(str string, size int, filter FilterFunc) <-chan map[Token]int {
	r := strings.NewReader(str)
	return t.TrainReaders([]io.ReadSeekCloser{nopCloser{r}}, size, filter)
}

func (t *Tokenizer) TrainReaders(readers []io.ReadSeekCloser, size int, filter FilterFunc) <-chan map[Token]int {
	ch := make(chan map[Token]int, 1)
	go func() {
		defer close(ch)

		t.dict = buildDict(readers)
		logging.Info("dict size: %d", t.dict.Size())

		seqs, cnt := t.loadSequence(readers)
		logging.Info("total tokens: %d", cnt)

		tokens := t.getTokens(seqs, filter)
		tokens = t.appendSpecialTokens(tokens)
		logging.Info("got %d tokens", len(tokens))

		ch <- tokens
		if len(tokens) >= size {
			return
		}

		var i int
		for {
			i++
			logging.Info("round %d", i)

			best := t.getStats(seqs, filter)
			if best == nil {
				return
			}
			logging.Info("round %d, best stats: (%s, %s)", i,
				fmtShow(best.word.string(t.dict)), fmtShow(best.next.string(t.dict)))

			t.merge(seqs, best)
			var total int
			for _, seq := range seqs {
				total += seq.Size()
			}
			logging.Info("round %d, %d tokens left", i, total)

			// for _, seq := range seqs {
			// 	fmt.Println(seq.String(dict))
			// }
			// return

			tokens = t.getTokens(seqs, filter)
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

func (t *Tokenizer) appendSpecialTokens(tokens map[Token]int) map[Token]int {
	for k := range t.specialTokens {
		k := buildToken(t.dict.Encode(k))
		if _, ok := tokens[k]; !ok {
			tokens[k] = 0
		}
	}
	return tokens
}

func (t *Tokenizer) Decode(tk Token) string {
	return tk.string(t.dict)
}
