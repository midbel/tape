package rw

import (
	"bufio"
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

type fileReader struct {
	reader          io.Reader
	remaining, size int
}

func (f *fileReader) Available() int {
	return f.remaining
}

func (f *fileReader) Size() int {
	return f.size
}

func NewReader(r io.Reader, s int) Reader {
	return &fileReader{
		reader:    r,
		remaining: s,
		size:      s,
	}
}

func (f *fileReader) Read(bs []byte) (int, error) {
	if f.remaining <= 0 {
		if m := f.size % 2; f.size != 0 && m == 1 {
			b := bufio.NewReader(f.reader)
			b.ReadByte()
			f.size = 0
		}
		return 0, io.EOF
	}
	if len(bs) > f.remaining {
		bs = bs[:f.remaining]
	}
	n, err := f.reader.Read(bs)
	f.remaining -= n
	return n, err
}

type fileWriter struct {
	writer          io.Writer
	remaining, size int
}

func NewWriter(w io.Writer, s int) Writer {
	return &fileWriter{
		writer:    w,
		remaining: s,
		size:      s,
	}
}

func (f *fileWriter) Available() int {
	return f.remaining
}

func (f *fileWriter) Size() int {
	return f.size
}

func (f *fileWriter) Write(bs []byte) (int, error) {
	if f.remaining < 0 {
		return 0, tape.ErrTooLong
	}
	var rest int
	switch {
	case len(bs) == 0:
		return 0, nil
	case len(bs) > f.size:
		rest = f.size
	case len(bs)-f.remaining < 0:
		rest = len(bs)
	default:
	}
	if rest > 0 {
		bs, rest = bs[:rest], 0
	}
	n, err := f.writer.Write(bs)
	f.remaining -= n
	if rest > 0 {
		return n, tape.ErrTooLong
	}
	return n, err
}
