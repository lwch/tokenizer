package tokenizer

import (
	"fmt"
	"io"
	"log"
	"strings"
	"testing"
)

const testData = "FloydHub is the fastest way to build, train and deploy deep learning models. Build deep learning models in the cloud. Train deep learning models."

func TestTrain(t *testing.T) {
	tk := New()
	tk.AddSpecialTokens(" ")
	for vocab := range tk.Train(testData, 32, nil) {
		var tokens []string
		for _, tk := range vocab.Tokens() {
			tokens = append(tokens, "["+string(tk)+"]")
		}
		fmt.Printf("%s\n", strings.Join(tokens, ", "))
	}
}

func benchmark() {
	tk := New()
	for range tk.Train(testData, 32, nil) {
	}
}

func BenchmarkTokenizer(b *testing.B) {
	log.SetOutput(io.Discard)
	for i := 0; i < b.N; i++ {
		benchmark()
	}
}
