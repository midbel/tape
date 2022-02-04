package cpio

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/midbel/rw"
	"github.com/midbel/tape"
)

var (
	magicASCII = []byte("070701")
	magicCRC   = []byte("070702")
)

const trailer = "TRAILER!!!"

const (
	blockSize = 512
	headerLen = 110
	fieldLen  = 8
	magicLen  = 6
)

type Writer struct {
	inner  io.Writer
	curr   io.Writer
	err    error
	blocks int64

	size    int
	written int
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{inner: w}
}

func (w *Writer) WriteHeader(h *tape.Header) error {
	if w.err = w.Flush(); w.err != nil {
		return w.err
	}
	if w.err = w.writeHeader(h, false); w.err != nil {
		return w.err
	}
	w.size = int(h.Size)
	w.curr = rw.LimitWriter(w.inner, h.Size)
	return w.err
}

func (w *Writer) Write(b []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.curr.Write(b)
	w.written += n
	w.blocks += int64(n)
	w.err = err
	return n, err
}

func (w *Writer) Flush() error {
	if w.curr == nil || w.err != nil {
		return w.err
	}
	if w.curr == nil || w.written < w.size {
		return tape.ErrTooShort
	}
	if mod := w.size % 4; mod > 0 {
		zs := make([]byte, 4-mod)
		_, w.err = w.inner.Write(zs)
	}
	w.reset()
	return w.err
}

func (w *Writer) Close() error {
	if w.err = w.Flush(); w.err != nil {
		return w.err
	}
	h := tape.Header{
		Filename: trailer,
	}
	if w.err = w.writeHeader(&h, true); w.err != nil {
		return w.err
	}
	w.pad()
	return w.err
}

func (w *Writer) pad() {
	var pad int
	if w.blocks < blockSize {
		pad = blockSize - int(w.blocks)
	} else {
		mod := w.blocks % blockSize
		if mod != 0 {
			pad = blockSize - int(mod)
		}
	}
	if pad == 0 {
		return
	}
	zs := make([]byte, pad)
	pad, w.err = w.inner.Write(zs)
	w.blocks = 0
}

func (w *Writer) writeHeader(h *tape.Header, trailing bool) error {
	var (
		buf bytes.Buffer
		z   = int64(len(h.Filename)) + 1
	)

	if !trailing {
		h.Mode |= 1 << 15
	}
	buf.Write(magicASCII)
	writeHeaderInt(&buf, h.Inode)
	writeHeaderInt(&buf, h.Mode)
	writeHeaderInt(&buf, h.Uid)
	writeHeaderInt(&buf, h.Gid)
	writeHeaderInt(&buf, h.Links)
	if t := h.ModTime; t.IsZero() {
		writeHeaderInt(&buf, 0)
	} else {
		writeHeaderInt(&buf, t.Unix())
	}
	writeHeaderInt(&buf, h.Size)
	writeHeaderInt(&buf, h.Major)
	writeHeaderInt(&buf, h.Minor)
	writeHeaderInt(&buf, h.RMajor)
	writeHeaderInt(&buf, h.RMinor)
	writeHeaderInt(&buf, z)
	writeHeaderInt(&buf, 0)
	writeFilename(&buf, h.Filename)

	w.blocks += headerLen + z
	if mod := w.blocks % 4; mod != 0 && !trailing {
		zs := make([]byte, 4-mod)
		n, _ := buf.Write(zs)
		w.blocks += int64(n)
	}
	_, w.err = io.Copy(w.inner, &buf)
	return w.err
}

func (w *Writer) reset() {
	w.size = 0
	w.written = 0
	w.curr = nil
}

type Reader struct {
	inner *bufio.Reader
	curr  io.Reader
	err   error

	read int
}

func NewReader(r io.Reader) *Reader {
	return &Reader{
		inner: bufio.NewReader(r),
	}
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
		r.discard(r.read)
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
	if r.curr != nil {
		io.Copy(io.Discard, r.curr)
	}
	h, err := r.next()
	if err != nil {
		r.err = err
		return nil, r.err
	}
	if h.Filename == trailer {
		return nil, io.EOF
	}
	r.read = 0
	r.curr = io.LimitReader(r.inner, h.Size)
	return h, nil
}

func (r *Reader) next() (*tape.Header, error) {
	var (
		h tape.Header
		z int64
	)
	if r.err = readMagic(r.inner); r.err != nil {
		return nil, r.err
	}
	h.Inode = readHeaderField(r.inner)
	h.Mode = readHeaderField(r.inner)
	h.Uid = readHeaderField(r.inner)
	h.Gid = readHeaderField(r.inner)
	h.Links = readHeaderField(r.inner)
	h.ModTime = readModTime(r.inner)
	h.Size = readHeaderField(r.inner)
	h.Major = readHeaderField(r.inner)
	h.Minor = readHeaderField(r.inner)
	h.RMajor = readHeaderField(r.inner)
	h.RMinor = readHeaderField(r.inner)
	z = readHeaderField(r.inner)
	h.Check = readHeaderField(r.inner)
	h.Filename = readFilename(r.inner, z)

	r.err = r.discard(int(headerLen + z))
	return &h, r.err
}

func (r *Reader) discard(n int) error {
	pad := n % 4
	if pad == 0 {
		return nil
	}
	n, err := r.inner.Discard(4 - pad)
	return err
}

func readMagic(r io.Reader) error {
	b := make([]byte, magicLen)
	if _, err := io.ReadFull(r, b); err != nil {
		return err
	}
	if bytes.Equal(b, magicCRC) || bytes.Equal(b, magicASCII) {
		return nil
	}
	fmt.Printf("%x - %x\n", b, magicASCII)
	return tape.ErrUnsupported
}

func readFilename(r io.Reader, n int64) string {
	bs := make([]byte, n)
	if _, err := io.ReadFull(r, bs); err != nil {
		return ""
	}
	return string(bs[:n-1])
}

func readModTime(r io.Reader) time.Time {
	when := readHeaderField(r)
	return time.Unix(when, 0)
}

func readHeaderField(r io.Reader) int64 {
	b := make([]byte, fieldLen)
	if _, err := io.ReadFull(r, b); err != nil {
		return -1
	}
	i, err := strconv.ParseInt(string(b), 16, 64)
	if err != nil {
		return -1
	}
	return i
}

func writeHeaderInt(w io.Writer, f int64) {
	fmt.Fprintf(w, "%08X", uint64(f))
}

func writeFilename(w io.Writer, f string) {
	fmt.Fprint(w, f+"\x00")
}
