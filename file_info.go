package simplefs

import (
	"os"
	"time"
)

type fileInfo struct {
	name  string
	size  int64
	isDir bool
}

func (info *fileInfo) Name() string {
	return info.name
}

func (info *fileInfo) Size() int64 {
	return info.size
}

func (info *fileInfo) Mode() os.FileMode {
	panic("Not implemented")
}

func (info *fileInfo) ModTime() time.Time {
	panic("Not implemented")
}

func (info *fileInfo) IsDir() bool {
	return info.isDir
}

func (info *fileInfo) Sys() interface{} {
	panic("Not implemented")
}

func (info *fileInfo) String() string {
	if info == nil {
		return "<nil>"
	}
	if info.isDir {
		return "dir(" + info.name + ")"
	}
	return "file(" + info.name + ")"
}
