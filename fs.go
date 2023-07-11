// Package simplefs provides a simple interface and implementation to
// work with a file system.

package simplefs

import (
	"fmt"
	"io"
)

var ErrNotFound = fmt.Errorf("not found")

type FS interface {
	Open(name string) (File, error)
	ReadDir(name string) ([]DirEntry, error)
	Create(name string) (io.WriteCloser, error)
	Append(name string) (io.WriteCloser, error)
}

type File interface {
	Read([]byte) (int, error)
	Close() error
	ReadDir(n int) ([]DirEntry, error)
}

type DirEntry interface {
	// Name returns the name of the file (or subdirectory) described by the entry.
	// This name is only the final element of the path (the base name), not the entire path.
	// For example, Name would return "hello.go" not "home/gopher/hello.go".
	Name() string

	// IsDir reports whether the entry describes a directory.
	IsDir() bool
}

type dirEntry struct {
	name  string
	isDir bool
}

func (entry *dirEntry) Name() string {
	return entry.name
}

func (entry *dirEntry) IsDir() bool {
	return entry.isDir
}

func (entry *dirEntry) String() string {
	if entry == nil {
		return "<nil>"
	}
	if entry.isDir {
		return "dir(" + entry.name + ")"
	}
	return "file(" + entry.name + ")"
}
