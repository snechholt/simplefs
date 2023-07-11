package simplefs

import "io"

type writeCloser struct {
	w       io.Writer
	closeFn func() error
}

func (w *writeCloser) Write(p []byte) (n int, err error) {
	return w.w.Write(p)
}

func (w *writeCloser) Close() error {
	if w.closeFn != nil {
		return w.closeFn()
	}
	return nil
}
