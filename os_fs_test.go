package simplefs

import (
	"fmt"
	"os"
	"path"
	"testing"
	"time"
)

func TestOsFileSystem(t *testing.T) {
	dir := path.Join(os.TempDir(), fmt.Sprintf("simplefs_%d", time.Now().UnixNano()))
	defer func() { _ = os.RemoveAll(dir) }()
	if msg := RunFileSystemTest(OsFS(dir)); msg != "" {
		t.Fatal(msg)
	}
}
