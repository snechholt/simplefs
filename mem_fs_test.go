package simplefs

import (
	"testing"
)

func TestInMemoryFileSystem(t *testing.T) {
	if msg := RunFileSystemTest(&MemFS{}); msg != "" {
		t.Fatal(msg)
	}
}
