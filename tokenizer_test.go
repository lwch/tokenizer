package tokenizer

import (
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestTrain(t *testing.T) {
	tk := New()
	str := "FloydHub is the fastest way to build, train and deploy deep learning models. Build deep learning models in the cloud. Train deep learning models."
	r := strings.NewReader(str)
	fmt.Println(tk.Train([]io.Reader{r}, 100))
}
