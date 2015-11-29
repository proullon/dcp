package dcp

import (
	"fmt"
	"os"
	"path"
	"testing"
)

func TestSum(t *testing.T) {
	filesum := `1d72fbf9b6045eae2245d042f7efdefb`
	file := path.Join(os.Getenv("GOPATH"), "src/github.com/proullon/dcp/test/1/foo/bar")

	sum, err := Sum(file)
	if err != nil {
		t.Fatalf("Unexpected sum error: %s\n", err)
	}

	if fmt.Sprintf("%x", sum) != filesum {
		t.Fatalf("Expected sum to be '%s', got '%s'", filesum, sum)
	}
}

func TestList(t *testing.T) {
	directory := path.Join(os.Getenv("GOPATH"), "src/github.com/proullon/dcp/test/1")

	files, err := List(directory)
	if err != nil {
		t.Fatalf("Unexpected list error: %s\n", err)
	}

	n := 0
	for _, f := range files {
		t.Logf("%s %x (%s)\n", f.Name, f.Sum, f.Path)
		if f.Name == "" {
			t.Fatalf("Name not set")
		}
		if f.Path == "" {
			t.Fatalf("Path not set")
		}
		n++
	}

	if n != 5 {
		t.Fatalf("Expected 5 files, got %d", n)
	}
}
