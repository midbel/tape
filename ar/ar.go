package ar

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"path/filepath"
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
	writeHeaderField(&buf, filepath.Base(h.Filename)+"/", 16)
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
	curr  io.Reader
	err   error

	read int
}

func NewReader(r io.Reader) (*Reader, error) {
	var (
		tmp    = bufio.NewReader(r)
		b, err = tmp.Peek(len(Magic))
	)
	if err != nil {
		return nil, err
	}
	if !bytes.Equal(b, Magic) {
		return nil, tape.ErrMagic
	}
	if _, err := tmp.Discard(len(b) + 1); err != nil {
		return nil, err
	}
	rs := Reader{
		inner: tmp,
	}
	return &rs, nil
}

func (r *Reader) Read(bs []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.curr == nil {
		return 0, tape.ErrRead
	}
	n, err := r.curr.Read(bs)
	r.read += n
	if errors.Is(err, io.EOF) {
		r.discard()
		r.curr = nil
	}
	if !errors.Is(err, io.EOF) {
		r.err = err
	}
	return n, err
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
	h, err := r.readHeader()
	if err != nil {
		r.err = err
		return nil, err
	}
	r.curr = io.LimitReader(r.inner, h.Length)
	r.read = 0
	return h, nil
}

func (r *Reader) readHeader() (*tape.Header, error) {
	var (
		h tape.Header
		b = make([]byte, len(linefeed))
	)
	if r.err = readFilename(r.inner, &h); r.err != nil {
		return nil, r.err
	}
	if r.err = readModTime(r.inner, &h); r.err != nil {
		return nil, r.err
	}
	if r.err = readFileInfos(r.inner, &h); r.err != nil {
		return nil, r.err
	}
	if _, r.err = r.inner.Read(b); r.err != nil || !bytes.Equal(b, linefeed) {
		return nil, r.err
	}
	return &h, r.err
}

func (r *Reader) discard() {
	if pad := r.read % 2; pad == 0 {
		return
	}
	r.inner.ReadByte()
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
	b, err := readHeaderField(r, 12)
	if err != nil {
		return err
	}
	when, err := strconv.ParseInt(string(b), 0, 64)
	if err != nil {
		return err
	}
	h.ModTime = time.Unix(when, 0).UTC()
	return nil
}

func readFileInfos(r io.Reader, h *tape.Header) error {
	if b, err := readHeaderField(r, 6); err != nil {
		return err
	} else {
		i, err := strconv.ParseInt(string(b), 0, 64)
		if err != nil {
			return err
		}
		h.Uid = i
	}
	if b, err := readHeaderField(r, 6); err != nil {
		return err
	} else {
		i, err := strconv.ParseInt(string(b), 0, 64)
		if err != nil {
			return err
		}
		h.Gid = i
	}
	if b, err := readHeaderField(r, 8); err != nil {
		return err
	} else {
		i, err := strconv.ParseInt(string(b), 8, 64)
		if err != nil {
			return err
		}
		h.Mode = i
	}
	if b, err := readHeaderField(r, 10); err != nil {
		return err
	} else {
		i, err := strconv.ParseInt(string(b), 0, 64)
		if err != nil {
			return err
		}
		h.Length = i
	}
	return nil
}

func readHeaderField(r io.Reader, n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(r, b); err != nil {
		return nil, err
	}
	return bytes.TrimSpace(b), nil
}

func writeHeaderField(w io.Writer, s string, n int) {
	io.WriteString(w, s)
	io.WriteString(w, strings.Repeat(" ", n-len(s)))
}
