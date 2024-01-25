package tokenizer

import "sync"

type words struct {
	sync.RWMutex
	data map[string]int
}

func newWords() *words {
	return &words{data: make(map[string]int)}
}

func (w *words) Size() int {
	return len(w.data)
}

func (w *words) Put(word string) {
	w.Lock()
	defer w.Unlock()
	w.data[word]++
}

func (w *words) Set(word string, freq int) {
	w.Lock()
	defer w.Unlock()
	w.data[word] = freq
}

func (w *words) Range(fn func(string, int)) {
	w.RLock()
	defer w.RUnlock()
	for k, v := range w.data {
		fn(k, v)
	}
}
