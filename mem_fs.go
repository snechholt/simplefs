package simplefs

import (
	"bytes"
	"fmt"
	"io"
	"path"
	"sort"
	"strings"
	"sync"
)

type MemFS struct {
	m map[string]*bytes.Buffer
	l sync.RWMutex
}

func (fs *MemFS) Size() int {
	fs.l.RLock()
	defer fs.l.RUnlock()
	var size int
	for _, buf := range fs.m {
		size += buf.Cap()
	}
	return size
}

func (fs *MemFS) Top() (name string, b []byte) {
	fs.l.RLock()
	defer fs.l.RUnlock()
	var max int
	for filename, buf := range fs.m {
		if n := buf.Len(); n > max {
			name, b = filename, buf.Bytes()
			max = n
		}
	}
	return name, b
}

func (fs *MemFS) Delete(name string) {
	fs.l.Lock()
	defer fs.l.Unlock()
	if fs.m != nil {
		delete(fs.m, name)
	}
}

func (fs *MemFS) HasFile(name string) bool {
	fs.l.RLock()
	defer fs.l.RUnlock()
	_, ok := fs.m[name]
	return ok
}

func (fs *MemFS) SetBytes(name string, b []byte) {
	w, _ := fs.Create(name)
	_, _ = w.Write(b)
	_ = w.Close()
}

func (fs *MemFS) TryGetBytes(name string) ([]byte, bool) {
	fs.l.RLock()
	defer fs.l.RUnlock()
	buf, ok := fs.m[name]
	if ok {
		return buf.Bytes(), true
	}
	return nil, false
}

func (fs *MemFS) GetBytes(name string) []byte {
	fs.l.RLock()
	defer fs.l.RUnlock()
	buf, ok := fs.m[name]
	if ok {
		return buf.Bytes()
	}
	return nil
}

func (fs *MemFS) SetString(name string, s string) {
	fs.SetBytes(name, []byte(s))
}

func (fs *MemFS) GetString(name string) string {
	fs.l.RLock()
	defer fs.l.RUnlock()
	buf, ok := fs.m[name]
	if ok {
		return string(buf.Bytes())
	}
	return ""
}

func (fs *MemFS) SetBytesMap(m map[string][]byte) {
	for name, b := range m {
		fs.SetBytes(name, b)
	}
}

func (fs *MemFS) SetStringsMap(m map[string]string) {
	for name, s := range m {
		fs.SetString(name, s)
	}
}

func (fs *MemFS) Create(name string) (io.WriteCloser, error) {
	fs.l.Lock()
	defer fs.l.Unlock()
	if fs.m == nil {
		fs.m = map[string]*bytes.Buffer{}
	}
	var buf bytes.Buffer
	fs.m[name] = &buf
	// TODO: the buffer shouldn't be added to m until the consumer closes the writer
	return &writeCloser{w: &buf}, nil
}

func (fs *MemFS) Append(name string) (io.WriteCloser, error) {
	fs.l.Lock()
	buf, ok := fs.m[name]
	fs.l.Unlock()
	if !ok {
		// TODO: the buffer shouldn't be added to m until the consumer closes the writer
		return fs.Create(name)
	}
	return &writeCloser{w: buf}, nil
}

func (fs *MemFS) Open(name string) (File, error) {
	fs.l.RLock()
	defer fs.l.RUnlock()
	buf, ok := fs.m[name]
	if ok {
		return &memFile{name: name, buf: bytes.NewBuffer(buf.Bytes())}, nil
	}
	for filepath := range fs.m {
		if strings.HasPrefix(filepath, name) {
			return &memDir{fs: fs, name: name}, nil
		}
	}
	return nil, ErrNotFound
}

func (fs *MemFS) ListFiles(dir string) ([]string, error) {
	fs.l.RLock()
	defer fs.l.RUnlock()

	if _, ok := fs.m[dir]; ok {
		return nil, ErrNotFound // If dir a file, return ErrNotFound
	}

	var names []string
	var found bool
	for name := range fs.m {
		if path.Dir(name) == dir {
			names = append(names, path.Base(name))
		} else if strings.HasPrefix(name, dir) {
			// If the file is nested within dir, this signals that dir is an actual
			// directory within the file system. We flag is at such, so we know not
			// to return ErrNotFound later.
			found = true
		}
	}

	// If no files were found in the directory, it means that we haven't written any
	if len(names) == 0 && !found {
		return nil, ErrNotFound
	}

	return names, nil
}

func (fs *MemFS) ReadDir(dir string) ([]DirEntry, error) {
	fs.l.RLock()
	defer fs.l.RUnlock()

	if _, ok := fs.m[dir]; ok {
		return nil, ErrNotFound // If dir a file, return ErrNotFound
	}

	n := len(dir)
	m := make(map[string]*dirEntry)
	for name := range fs.m {
		if strings.HasPrefix(name, dir) {
			sub := name[n+1:]
			split := strings.SplitN(sub, "/", 2)
			entry := &dirEntry{name: split[0], isDir: len(split) > 1}
			m[entry.name] = entry
		}
	}

	if len(m) == 0 {
		return nil, ErrNotFound
	}

	entries := make([]DirEntry, 0, len(m))
	for _, entry := range m {
		entries = append(entries, entry)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })

	return entries, nil
}

type memFile struct {
	name string
	buf  *bytes.Buffer
}

func (f *memFile) Read(p []byte) (n int, err error) {
	return f.buf.Read(p)
}

func (f *memFile) Close() error {
	return nil
}

func (f *memFile) ReadDir(n int) ([]DirEntry, error) {
	return nil, fmt.Errorf("cannot ReadDir '%s'. Path is a file", f.name)
}

type memDir struct {
	fs             *MemFS
	name           string
	readDirEntries []DirEntry
}

func (dir *memDir) Read(p []byte) (n int, err error) {
	return 0, fmt.Errorf("cannot read '%s'. Path is a directory", dir.name)
}

func (dir *memDir) Close() error {
	return nil
}

func (dir *memDir) ReadDir(n int) ([]DirEntry, error) {
	if dir.readDirEntries == nil {
		entries, err := dir.fs.ReadDir(dir.name)
		if err != nil {
			return nil, err
		}
		dir.readDirEntries = entries
	}

	if len(dir.readDirEntries) == 0 {
		if n < 0 {
			return dir.readDirEntries, nil
		}
		return dir.readDirEntries, io.EOF
	}

	size := n
	if size < 0 || size > len(dir.readDirEntries) {
		size = len(dir.readDirEntries)
	}

	entries := dir.readDirEntries[:size]
	dir.readDirEntries = dir.readDirEntries[size:]

	return entries, nil
}
