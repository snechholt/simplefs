package simplefs

import (
	"io"
	"io/ioutil"
	"os"
	"path"
)

type osFs struct {
	dir string
}

func OsFS(dir string) FS {
	return &osFs{dir: dir}
}

func (fs *osFs) Create(name string) (io.WriteCloser, error) {
	p := path.Join(fs.dir, name)
	if err := os.MkdirAll(path.Dir(p), 0666); err != nil {
		return nil, err
	}
	return os.Create(p)
}

func (fs *osFs) Append(name string) (io.WriteCloser, error) {
	p := path.Join(fs.dir, name)
	if err := os.MkdirAll(path.Dir(p), 0666); err != nil {
		return nil, err
	}
	return os.OpenFile(p, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)
}

func (fs *osFs) Open(name string) (File, error) {
	f, err := os.Open(path.Join(fs.dir, name))
	if err != nil && os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	return &osFile{f}, err
}

func (fs *osFs) ListFiles(dir string) ([]string, error) {
	info, err := ioutil.ReadDir(path.Join(fs.dir, dir))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	var names []string
	for _, f := range info {
		if !f.IsDir() {
			names = append(names, f.Name())
		}
	}
	return names, nil
}

func (fs *osFs) ReadDir(name string) ([]DirEntry, error) {
	osInfos, err := ioutil.ReadDir(path.Join(fs.dir, name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	fileInfos := make([]os.FileInfo, len(osInfos))
	for i, info := range osInfos {
		fileInfos[i] = &fileInfo{name: info.Name(), isDir: info.IsDir()}
	}
	dirEntries := make([]DirEntry, len(fileInfos))
	for i, info := range fileInfos {
		dirEntries[i] = &dirEntry{name: info.Name(), isDir: info.IsDir()}
	}
	return dirEntries, err
}

type osFile struct {
	f *os.File
}

func (f *osFile) Read(p []byte) (n int, err error) {
	return f.f.Read(p)
}

func (f *osFile) Close() error {
	return f.f.Close()
}

func (f *osFile) ReadDir(n int) ([]DirEntry, error) {
	fileInfos, err := f.f.Readdir(n)
	if err != nil && os.IsNotExist(err) {
		return nil, ErrNotFound
	}
	dirEntries := make([]DirEntry, len(fileInfos))
	for i, info := range fileInfos {
		dirEntries[i] = &dirEntry{name: info.Name(), isDir: info.IsDir()}
	}
	return dirEntries, err
}
