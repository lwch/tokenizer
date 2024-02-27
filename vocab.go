package tokenizer

import (
	"io"
	"sort"
)

type BSToken [maxSeq]byte

func (tk BSToken) Len() int {
	for i := 0; i < maxSeq; i++ {
		if tk[i] == 0 {
			return i
		}
	}
	return maxSeq
}

func (tk BSToken) Bytes() []byte {
	var ret []byte
	for i := 0; i < maxSeq; i++ {
		if tk[i] == 0 {
			break
		}
		ret = append(ret, tk[i])
	}
	return ret
}

type Vocab struct {
	id2words map[int]BSToken
	words2id map[BSToken]int
	max      int
}

func newVocab(words []Token, dict *dict) *Vocab {
	sort.Slice(words, func(i, j int) bool {
		if words[i].Len() == words[j].Len() {
			return less(words[i], words[j])
		}
		return words[i].Len() < words[j].Len()
	})
	id2words := make(map[int]BSToken)
	words2id := make(map[BSToken]int)
	var max int
	for i, word := range words {
		var token BSToken
		word := word.bytes(dict)
		copy(token[:], word)
		id2words[i] = token
		words2id[token] = i
		if len(word) > max {
			max = len(word)
		}
	}
	return &Vocab{
		id2words: id2words,
		words2id: words2id,
	}
}

func (v *Vocab) Len() int {
	return len(v.id2words)
}

func (v *Vocab) Tokens() [][]byte {
	ret := make([][]byte, len(v.id2words))
	var ids []int
	for id := range v.id2words {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for id := range ids {
		ret[id] = v.id2words[id].Bytes()
	}
	return ret
}

func (v *Vocab) ID(token BSToken) int {
	return v.words2id[token]
}

func (v *Vocab) Token(id int) BSToken {
	return v.id2words[id]
}

func (v *Vocab) Encode(str string, unset int) <-chan int {
	ret := make(chan int, 100)
	go func() {
		data := []byte(str)
		var tmp []byte
		for _, b := range data {
			tmp = append(tmp, b)
			if len(tmp) >= v.max {
				tk, size := v.encode(tmp, unset)
				ret <- tk
				tmp = tmp[size:]
			}
		}
		for len(tmp) > 0 {
			tk, size := v.encode(tmp, unset)
			ret <- tk
			tmp = tmp[size:]
		}
	}()
	return ret
}

func (v *Vocab) encode(data []byte, unset int) (int, int) {
	for i := len(data); i > 0; i-- {
		tk := BSToken{}
		copy(tk[:], data[:i])
		if id, ok := v.words2id[tk]; ok {
			return id, i
		}
	}
	return unset, 1
}

func (v *Vocab) Decode(tokens []int) string {
	var ret []byte
	for _, id := range tokens {
		ret = append(ret, v.id2words[id].Bytes()...)
	}
	return string(ret)
}

func (v *Vocab) WriteTo(w io.Writer) (int64, error) {
	var total int64
	for _, tk := range v.id2words {
		n, err := w.Write([]byte{byte(tk.Len())})
		if err != nil {
			return total, err
		}
		total += int64(n)
		n, err = w.Write(tk.Bytes())
		if err != nil {
			return total, err
		}
		total += int64(n)
	}
	return total, nil
}

func (v *Vocab) ReadFrom(r io.Reader) (int64, error) {
	v.id2words = make(map[int]BSToken)
	v.words2id = make(map[BSToken]int)
	v.max = 0
	var total int64
	for {
		var size byte
		_, err := r.Read([]byte{size})
		if err != nil {
			if err == io.EOF {
				return total, nil
			}
			return total, err
		}
		total++
		var tk BSToken
		_, err = r.Read(tk[:size])
		if err != nil {
			return total, err
		}
		total += int64(size)
		v.id2words[len(v.id2words)] = tk
		v.words2id[tk] = len(v.id2words) - 1
		if int(size) > v.max {
			v.max = int(size)
		}
	}
}
