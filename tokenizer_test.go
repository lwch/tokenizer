package tokenizer

import (
	"fmt"
	"testing"
)

func TestTrain(t *testing.T) {
	tk := New()
	str := "FloydHub is the fastest way to build, train and deploy deep learning models. Build deep learning models in the cloud. Train deep learning models."
	for vocab := range tk.Train(str, 32) {
		fmt.Println(vocab)
	}
}
