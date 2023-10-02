package simplefs

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
)

type MemFS struct {
	root *dirNode
	l    sync.RWMutex
}

func (fs *MemFS) SetBytes(name string, b []byte) {
	w, _ := fs.Create(name)
	_, _ = w.Write(b)
	_ = w.Close()
}

func (fs *MemFS) SetString(name string, s string) {
	fs.SetBytes(name, []byte(s))
}

func (fs *MemFS) init() {
	fs.l.Lock()
	if fs.root == nil {
		fs.root = &dirNode{}
	}
	fs.l.Unlock()
}

func (fs *MemFS) Create(name string) (io.WriteCloser, error) {
	fs.init()
	fs.l.Lock()
	defer fs.l.Unlock()
	var buf bytes.Buffer
	addNode := func() error {
		fs.l.Lock()
		defer fs.l.Unlock()
		b := getBytes(&buf)
		node := fs.root.GetOrAdd(b, nameToPath(name)...)
		node.B = b
		return nil
	}
	return &writeCloser{w: &buf, closeFn: addNode}, nil
}

func (fs *MemFS) Append(name string) (io.WriteCloser, error) {
	fs.init()
	fs.l.Lock()
	got := fs.root.Get(nameToPath(name)...)
	fs.l.Unlock()
	if got == nil {
		return fs.Create(name)
	}
	var buf bytes.Buffer
	updateNode := func() error {
		fs.l.Lock()
		defer fs.l.Unlock()
		b := getBytes(&buf)
		got.B = append(got.B, b...)
		return nil
	}
	return &writeCloser{w: &buf, closeFn: updateNode}, nil
}

func (fs *MemFS) Open(name string) (File, error) {
	fs.init()
	fs.l.RLock()
	defer fs.l.RUnlock()
	node := fs.root.Get(nameToPath(name)...)
	if node == nil {
		return nil, ErrNotFound
	}
	if node.IsDirectory() {
		return &memDir{fs: fs, name: name}, nil
	} else {
		return &memFile{name: name, buf: bytes.NewBuffer(node.B)}, nil
	}
}

func (fs *MemFS) ListFiles(dir string) ([]string, error) {
	fs.init()
	fs.l.RLock()
	defer fs.l.RUnlock()

	node := fs.root.Get(nameToPath(dir)...)

	if node != nil && !node.IsDirectory() {
		return nil, ErrNotFound // If dir a file, return ErrNotFound
	}

	var names []string
	node.DFS(func(node *dirNode) {
		names = append(names, node.Path())
	})

	return names, nil
}

func (fs *MemFS) ReadDir(dir string) ([]DirEntry, error) {
	fs.init()
	fs.l.RLock()
	defer fs.l.RUnlock()

	node := fs.root.Get(nameToPath(dir)...)

	if node == nil || !node.IsDirectory() {
		return nil, ErrNotFound // If dir a file, return ErrNotFound
	}

	entries := make([]DirEntry, len(node.Children))
	for i, child := range node.Children {
		entries[i] = &dirEntry{name: child.Name, isDir: child.IsDirectory()}
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

type dirNode struct {
	Name     string
	Parent   *dirNode
	Children dirNodeSlice
	B        []byte
}

func (node *dirNode) Level() int {
	var level int
	parent := node.Parent
	for parent != nil {
		level++
		parent = parent.Parent
	}
	return level
}

func (node *dirNode) IsDirectory() bool {
	return node.B == nil
}

func (node *dirNode) Get(path ...string) *dirNode {
	if len(path) == 0 {
		panic(":(")
	}
	var next *dirNode
	p := path[0]
	switch p {
	case ".":
		next = node
	case "..":
		next = node.Parent
	default:
		next = node.Children.Get(p)
	}
	if next == nil {
		return nil
	}
	if len(path) > 1 {
		return next.Get(path[1:]...)
	}
	return next
}

func (node *dirNode) Path() string {
	if node.Parent == nil {
		return node.Name
	}
	return node.Parent.Path() + "/" + node.Name
}

func (node *dirNode) AddDescendant(b []byte, path ...string) *dirNode {
	childName := path[0]
	if len(path) > 1 {
		child := node.Children.Get(childName)
		if child == nil {
			child = node.AddChild(childName, nil)
		}
		return child.AddDescendant(b, path[1:]...)
	}
	child := node.Children.Get(childName)
	if child == nil {
		child = node.AddChild(childName, b)
	}
	return child
}

func (node *dirNode) AddChild(name string, b []byte) *dirNode {
	child := &dirNode{Name: name, Parent: node, B: b}
	node.Children = append(node.Children, child)
	sort.Sort(node.Children)
	return child
}

func (node *dirNode) GetOrAdd(b []byte, path ...string) *dirNode {
	if got := node.Get(path...); got != nil {
		return got
	}
	return node.AddDescendant(b, path...)
}

func (node *dirNode) DFS(fn func(node *dirNode)) {
	fn(node)
	for _, child := range node.Children {
		child.DFS(fn)
	}
}

func (node *dirNode) String() string {
	var sb strings.Builder
	node.DFS(func(child *dirNode) {
		_, _ = fmt.Fprintf(&sb, "%s%s\n", strings.Repeat("\t", child.Level()), child.toString())
	})
	return sb.String()
}

func (node *dirNode) toString() string {
	if node.IsDirectory() {
		return fmt.Sprintf("dir(%s)", node.Name)
	}
	return fmt.Sprintf("file(%s)", node.Name)
	// return fmt.Sprintf("{ ID:%d, Code:%s Name:%s }", node.EntityID(), node.GetCode(), node.GetName())
}

type dirNodeSlice []*dirNode

func (s dirNodeSlice) Len() int           { return len(s) }
func (s dirNodeSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }
func (s dirNodeSlice) Less(i, j int) bool { return s[i].Name < s[j].Name }

func (s dirNodeSlice) Get(name string) *dirNode {
	for _, node := range s {
		if node.Name == name {
			return node
		}
	}
	return nil
}

func nameToPath(name string) []string {
	return strings.Split(name, "/")
}

func getBytes(buf *bytes.Buffer) []byte {
	b := buf.Bytes()
	if b != nil {
		return b
	}
	return make([]byte, 0)
}
