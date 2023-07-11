package simplefs

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
)

func RunFileSystemTest(fs FS) string {
	type File struct {
		Name     string
		Contents []byte
	}

	var t runner

	assertFileContents := func(files ...File) {
		for _, f := range files {
			r, err := fs.Open(f.Name)
			if err != nil {
				t.Fatalf("Open(%s) error: %v", f.Name, err)
			}
			b, err := ioutil.ReadAll(r)
			if err != nil {
				t.Fatalf("%s: Read() error: %v", f.Name, err)
			}
			if bytes.Compare(b, f.Contents) != 0 {
				t.Fatalf("%s: Wrong file contents: %v", f.Name, b)
			}
		}
	}

	// Opening non-existing file returns ErrNotFound
	t.Run("Opening non-existent file", func() {
		r, err := fs.Open("file.txt")
		if err != ErrNotFound {
			t.Fatalf("Wrong error returned: %v", err)
		}
		if r != nil {
			t.Fatalf("Non-nil reader returned")
		}
	})

	// Creating a file and reading it back
	file1 := File{Name: "file1", Contents: []byte{11, 12, 13}}
	t.Run("Create file", func() {
		w, err := fs.Create(file1.Name)
		if err != nil {
			t.Fatalf("Create() error: %v", err)
		}
		if _, err := w.Write(file1.Contents); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close() error: %v", err)
		}

		assertFileContents(file1)
	})

	// Overwrite with new content
	file1.Contents = []byte{12, 13, 14}
	t.Run("Overwrite file", func() {
		w, err := fs.Create(file1.Name)
		if err != nil {
			t.Fatalf("Create() error: %v", err)
		}
		if _, err := w.Write(file1.Contents); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close() error: %v", err)
		}

		assertFileContents(file1)
	})

	// Assert that we read the correct file by name
	file2 := File{Name: "file2", Contents: []byte{21, 22, 23}}
	t.Run("Create another file", func() {
		w, err := fs.Create(file2.Name)
		if err != nil {
			t.Fatalf("Create() error: %v", err)
		}
		if _, err := w.Write(file2.Contents); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close() error: %v", err)
		}

		assertFileContents(file1, file2)
	})

	// Append with additional bytes
	append1 := []byte{15, 16}
	file1.Contents = append(file1.Contents, append1...)
	t.Run("Append to existing file", func() {
		w, err := fs.Append(file1.Name)
		if err != nil {
			t.Fatalf("Append() error: %v", err)
		}
		if _, err := w.Write(append1); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close() error: %v", err)
		}

		assertFileContents(file1, file2)
	})

	file3 := File{Name: "file3", Contents: []byte{31, 32, 33}}
	t.Run("Append to non-existing file", func() {
		w, err := fs.Append(file3.Name)
		if err != nil {
			t.Fatalf("Append() error: %v", err)
		}
		if _, err := w.Write(file3.Contents); err != nil {
			t.Fatalf("Write() error: %v", err)
		}
		if err := w.Close(); err != nil {
			t.Fatalf("Close() error: %v", err)
		}

		assertFileContents(file1, file2, file3)
	})

	// Now that we've covered the primitive Create, Append and Open we can use some utility
	// functions for efficiency.
	create := func(f File) error {
		w, err := fs.Create(f.Name)
		if err != nil {
			return err
		}
		if len(f.Contents) > 0 {
			if _, err := w.Write(f.Contents); err != nil {
				return err
			}
		}
		return w.Close()
	}

	t.Run("Create empty file", func() {
		f := File{Name: "empty"}
		if err := create(f); err != nil {
			t.Fatalf("Error creating file: %v", err)
		}
		assertFileContents(f)
	})

	t.Run("ReadDir", func() {
		files := []string{
			"dir1/file1A",
			"dir1/file1B",
			"dir2/file2A",
			"dir2/file2B",
			// Add a subdirectory inside dir2. This allows us to test that the logic does
			// not recurse into subdirectories
			"dir2/dir3/file3A",
			"dir2/dir3/file3B",
			// Add a directory with single directory inside it (which contains a file)
			// This allows us to test that we do not return ErrNotFound when listing in
			// directories with no files in them.
			"dir4/dir5/file",
		}
		for _, filename := range files {
			f := File{Name: filename}
			if err := create(f); err != nil {
				t.Fatalf("Error creating file: %v", err)
			}
		}
		dir := func(name string) *dirEntry { return &dirEntry{name: name, isDir: true} }
		file := func(name string) *dirEntry { return &dirEntry{name: name, isDir: false} }
		tests := map[string][]DirEntry{
			"dir1":      {file("file1A"), file("file1B")},
			"dir2":      {dir("dir3"), file("file2A"), file("file2B")},
			"dir2/dir3": {file("file3A"), file("file3B")},
			"dir4":      {dir("dir5")},
		}

		t.Run("File.ReadDir", func() {
			for _, n := range []int{-1, 1, 2, 3, 4, 5} {
				for name, want := range tests {
					dir, err := fs.Open(name)
					if err != nil {
						t.Fatalf("Open(%s) returned error: %v", name, err)
					}
					var got []DirEntry
					for {
						entries, err := dir.ReadDir(n)
						got = append(got, entries...)
						if n == -1 {
							if err != nil {
								t.Fatalf("Open(%s).ReadDir(%d) returned error: %v", name, n, err)
							}
							break
						} else {
							if err == io.EOF {
								break
							}
							if err != nil {
								t.Fatalf("Open(%s).ReadDir(%d) returned error: %v", name, n, err)
							}
						}
					}
					if !compareDirEntries(got, want) {
						t.Fatalf("Open(%s).ReadDir(%d) returned %v, want %v", name, n, got, want)
					}
				}
			}

			t.Run("On file", func() {
				dir, err := fs.Open(file1.Name)
				if err != nil {
					t.Fatalf("Open(%s) returned error: %v", file1.Name, err)
				}
				if _, err = dir.ReadDir(-1); err == nil {
					t.Fatalf("ReadDir() returned nil error")
				}
			})
		})

		t.Run("fs.ReadDir", func() {
			for name, want := range tests {
				got, err := fs.ReadDir(name)
				if err != nil {
					t.Fatalf("fs.ReadDir(%v) returned error: %v", name, err)
				}
				if !compareDirEntries(got, want) {
					t.Fatalf("fs.ReadDir(%v) returned %v, want %v", name, got, want)
				}
			}

			t.Run("On file", func() {
				dir, err := fs.Open(file1.Name)
				if err != nil {
					t.Fatalf("Open(%s) returned error: %v", file1.Name, err)
				}
				if _, err = dir.ReadDir(-1); err == nil {
					t.Fatalf("ReadDir() returned nil error")
				}
			})

			t.Run("On non-existent directory", func() {
				_, err := fs.ReadDir("non-existent-dir")
				if err != ErrNotFound {
					t.Fatalf("Wrong error returned: %v", err)
				}
			})
		})

	})

	return t.msg
}

type runner struct {
	path []string
	msg  string
}

func (r *runner) Run(name string, fn func()) {
	if r.msg != "" {
		return
	}
	defer func() { r.path = r.path[:len(r.path)-1] }()
	defer func() {
		if msg := recover(); msg != nil {
			r.msg = fmt.Sprintf("'%s': %s", strings.Join(r.path, "/"), msg.(string))
		}

	}()
	r.path = append(r.path, name)
	fn()
}

func (r *runner) Fatalf(s string, args ...interface{}) {
	// r.msg = fmt.Sprintf(s, args...)
	panic(fmt.Sprintf(s, args...))
}

func compareDirEntries(entries1, entries2 []DirEntry) bool {
	if len(entries1) != len(entries2) {
		return false
	}
	for i := range entries1 {
		e1, e2 := entries1[i], entries2[i]
		ok := e1.Name() == e2.Name() && e1.IsDir() == e2.IsDir()
		if !ok {
			return false
		}
	}
	return true
}
