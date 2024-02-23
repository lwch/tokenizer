package tokenizer

import (
	"fmt"
	"io"
	"log"
	"testing"
)

const testData = "FloydHub is the fastest way to build, train and deploy deep learning models. Build deep learning models in the cloud. Train deep learning models."

func TestTrain(t *testing.T) {
	tk := New()
	tk.AddSpecialTokens(" ")
	for vocab := range tk.Train(testData, 32, 6, nil) {
		fmt.Println(vocab)
	}
}

func benchmark() {
	tk := New()
	for range tk.Train(testData, 32, 6, nil) {
	}
}

func BenchmarkTokenizer(b *testing.B) {
	log.SetOutput(io.Discard)
	for i := 0; i < b.N; i++ {
		benchmark()
	}
}
