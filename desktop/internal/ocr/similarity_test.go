package ocr

import (
	"encoding/json"
	"math/rand"
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestLongTextAutojunk(t *testing.T) {
	if Similar("甲甲"+strings.Repeat("中", 210), "乙乙"+strings.Repeat("中", 210)) {
		t.Fatal("ignored Python autojunk")
	}
	if !Similar(strings.Repeat("中", 210)+"甲", strings.Repeat("中", 210)+"乙") {
		t.Fatal("lost popular prefix extension")
	}
}

func TestPythonSimilarityConformance(t *testing.T) {
	python := os.Getenv("JUNSHI_TEST_PYTHON")
	if python == "" {
		t.Skip("set JUNSHI_TEST_PYTHON for difflib comparison")
	}
	type pair struct {
		A string `json:"a"`
		B string `json:"b"`
	}
	pairs := []pair{}
	random := rand.New(rand.NewSource(13))
	alphabet := []rune("甲乙中你我，abc")
	for _, length := range []int{2, 20, 80, 199, 200, 250, 500} {
		for sample := 0; sample < 20; sample++ {
			a := make([]rune, length)
			for i := range a {
				a[i] = alphabet[random.Intn(len(alphabet))]
			}
			b := append([]rune{}, a...)
			for i := 0; i < length/4; i++ {
				b[random.Intn(len(b))] = alphabet[random.Intn(len(alphabet))]
			}
			pairs = append(pairs, pair{string(a), string(b)})
		}
	}
	encoded, _ := json.Marshal(pairs)
	program := `import sys,json,difflib
def similar(a,b):
 if a!=b and (a in b or b in a) and abs(len(a)-len(b))>=4:return False
 if a==b or difflib.SequenceMatcher(None,a,b).ratio()>=0.75:return True
 return len(a)==len(b)>=3 and sum(x!=y for x,y in zip(a,b))<=1
print(json.dumps([similar(p['a'],p['b']) for p in json.load(sys.stdin)]))`
	cmd := exec.Command(python, "-c", program)
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")
	cmd.Stdin = strings.NewReader(string(encoded))
	output, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	var expected []bool
	if err = json.Unmarshal(output, &expected); err != nil {
		t.Fatal(err)
	}
	for i, pair := range pairs {
		if actual := Similar(pair.A, pair.B); actual != expected[i] {
			t.Fatalf("case %d length %d: got %v want %v; a=%q b=%q", i, len([]rune(pair.A)), actual, expected[i], pair.A, pair.B)
		}
	}
}
