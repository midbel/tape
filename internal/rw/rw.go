package rw

import (
	"io"

	"github.com/midbel/tape"
)

type Size interface {
	Available() int
	Size() int
}

type Reader interface {
	io.Reader
	Size
}

type Writer interface {
	io.Writer
	Size
}

type fileWriter struct {
	writer io.Writer
	remain int
	size   int
}

func NewWriter(w io.Writer, s int) Writer {
	return &fileWriter{
		writer: w,
		remain: s,
		size:   s,
	}
}

func (f *fileWriter) Available() int {
	return f.remain
}

func (f *fileWriter) Size() int {
	return f.size
}

func (f *fileWriter) Write(bs []byte) (int, error) {
	if f.remain < 0 {
		return 0, tape.ErrTooLong
	}
	var rest int
	switch z := len(bs); {
	case z == 0:
		return 0, nil
	case z > f.size:
		rest = f.size
	case z-f.remain < 0:
		rest = z
	default:
	}
	if rest > 0 {
		bs, rest = bs[:rest], 0
	}
	n, err := f.writer.Write(bs)
	f.remain -= n
	if rest > 0 {
		return n, tape.ErrTooLong
	}
	return n, err
}
