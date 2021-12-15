package ar

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/tape"
	"github.com/midbel/tape/internal/rw"
)

var (
	Magic    = []byte("!<arch>")
	linefeed = []byte{0x60, 0x0A}
)

type Writer struct {
	inner io.Writer
	curr  rw.Writer
	err   error
}

func NewWriter(w io.Writer) (*Writer, error) {
	if _, err := w.Write(Magic); err != nil {
		return nil, err
	}
	if _, err := w.Write(linefeed[1:]); err != nil {
		return nil, err
	}
	return &Writer{inner: w}, nil
}

func (w *Writer) WriteHeader(h *tape.Header) error {
	if w.err != nil {
		return w.err
	}
	if err := w.Flush(); err != nil {
		w.err = err
		return err
	}

	var buf bytes.Buffer
	writeHeaderField(&buf, path.Base(h.Filename)+"/", 16)
	writeHeaderField(&buf, strconv.FormatInt(h.ModTime.Unix(), 10), 12)
	writeHeaderField(&buf, strconv.FormatInt(h.Uid, 10), 6)
	writeHeaderField(&buf, strconv.FormatInt(h.Gid, 10), 6)
	writeHeaderField(&buf, strconv.FormatInt(h.Mode, 8), 8)
	writeHeaderField(&buf, strconv.FormatInt(h.Length, 10), 10)
	buf.Write(linefeed)

	if _, w.err = io.Copy(w.inner, &buf); w.err != nil {
		return w.err
	}
	w.curr = rw.NewWriter(w.inner, int(h.Length))
	return nil
}

func (w *Writer) Flush() error {
	if w.curr == nil || w.err != nil {
		return w.err
	}
	if w.curr == nil || w.curr.Available() > 0 {
		return tape.ErrTooShort
	}
	if mod := w.curr.Size() % 2; mod == 1 {
		_, w.err = w.inner.Write(linefeed[1:])
	}
	w.curr = nil
	return w.err
}

func (w *Writer) Write(bs []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.curr.Write(bs)
	if err != nil && !errors.Is(err, tape.ErrTooLong) {
		w.err = err
	}
	return n, w.err
}

func (w *Writer) Close() error {
	return w.Flush()
}

type Reader struct {
	inner *bufio.Reader
	curr  rw.Reader
	err   error
}

func NewReader(r io.Reader) (*Reader, error) {
	var (
		rs      = bufio.NewReader(r)
		bs, err = rs.Peek(len(Magic))
	)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(bs, Magic) {
		return nil, tape.ErrMagic
	}
	if _, err := rs.Discard(len(bs) + 1); err != nil {
		return nil, err
	}
	return &Reader{inner: rs}, nil
}

func (r *Reader) Next() (*tape.Header, error) {
	if r.err != nil {
		return nil, r.err
	}
	return r.next()
}

func (r *Reader) next() (*tape.Header, error) {
	if r.curr != nil {
		io.Copy(io.Discard, r.curr)
	}

	var h tape.Header
	if r.err = readFilename(r.inner, &h); r.err != nil {
		return nil, r.err
	}
	if r.err = readModTime(r.inner, &h); r.err != nil {
		return nil, r.err
	}
	if r.err = readFileInfos(r.inner, &h); r.err != nil {
		return nil, r.err
	}
	bs := make([]byte, len(linefeed))
	if _, r.err = r.inner.Read(bs); r.err != nil || !bytes.Equal(bs, linefeed) {
		return nil, r.err
	}
	r.curr = rw.NewReader(r.inner, int(h.Length))
	return &h, r.err
}

func (r *Reader) Read(bs []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.curr.Read(bs)
	if err != nil && !errors.Is(err, io.EOF) {
		r.err = err
	}
	return n, err
}

func readFilename(r io.Reader, h *tape.Header) error {
	bs, err := readHeaderField(r, 16)
	if err != nil {
		return err
	}
	h.Filename = strings.TrimRight(string(bs), "/")
	return nil
}

func readModTime(r io.Reader, h *tape.Header) error {
	bs, err := readHeaderField(r, 12)
	if err != nil {
		return err
	}
	i, err := strconv.ParseInt(string(bs), 0, 64)
	if err != nil {
		return err
	}
	h.ModTime = time.Unix(i, 0)
	return nil
}

func readFileInfos(r io.Reader, h *tape.Header) error {
	if bs, err := readHeaderField(r, 6); err != nil {
		return err
	} else {
		i, err := strconv.ParseInt(string(bs), 0, 64)
		if err != nil {
			return err
		}
		h.Uid = i
	}
	if bs, err := readHeaderField(r, 6); err != nil {
		return err
	} else {
		i, err := strconv.ParseInt(string(bs), 0, 64)
		if err != nil {
			return err
		}
		h.Gid = i
	}
	if bs, err := readHeaderField(r, 8); err != nil {
		return err
	} else {
		i, err := strconv.ParseInt(string(bs), 8, 64)
		if err != nil {
			return err
		}
		h.Mode = i
	}
	if bs, err := readHeaderField(r, 10); err != nil {
		return err
	} else {
		i, err := strconv.ParseInt(string(bs), 0, 64)
		if err != nil {
			return err
		}
		h.Length = i
	}
	return nil
}

func readHeaderField(r io.Reader, n int) ([]byte, error) {
	bs := make([]byte, n)
	if _, err := io.ReadFull(r, bs); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(bs), nil
}

func writeHeaderField(w io.Writer, s string, n int) {
	io.WriteString(w, s)
	io.WriteString(w, strings.Repeat(" ", n-len(s)))
}
