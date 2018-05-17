package cpio

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"strconv"
	"time"

	"github.com/midbel/tape"
	"github.com/midbel/tape/internal/rw"
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
	curr   rw.Writer
	err    error
	blocks int64
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{inner: w}
}

func (w *Writer) WriteHeader(h *tape.Header) error {
	if err := w.Flush(); err != nil {
		w.err = err
		return err
	}
	if err := w.writeHeader(h, false); err != nil {
		w.err = err
		return w.err
	}
	w.curr = rw.NewWriter(w.inner, int(h.Length))
	return w.err
}

func (w *Writer) Write(bs []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.curr.Write(bs)
	if err != nil && err != tape.ErrTooLong {
		w.err = err
	}
	return n, err
}

func (w *Writer) Flush() error {
	if w.curr == nil {
		return nil
	}
	if w.err != nil {
		return w.err
	}
	if w.curr == nil || w.curr.Available() > 0 {
		return tape.ErrTooShort
	}
	if mod := w.curr.Size() % 4; mod > 0 {
		zs := make([]byte, 4-mod)
		_, w.err = w.inner.Write(zs)
	}
	w.curr = nil
	return w.err
}

func (w *Writer) Close() error {
	if err := w.Flush(); err != nil {
		w.err = err
		return w.err
	}
	h := tape.Header{Filename: trailer}
	if err := w.writeHeader(&h, true); err != nil {
		w.err = err
		return err
	}
	if mod := w.blocks % blockSize; mod != 0 {
		zs := make([]byte, blockSize-mod)
		_, w.err = w.inner.Write(zs)
	}
	return w.err
}

func (w *Writer) writeHeader(h *tape.Header, trailing bool) error {
	buf := new(bytes.Buffer)
	z := int64(len(h.Filename)) + 1

	buf.Write(magicASCII)
	writeHeaderInt(buf, h.Inode)
	writeHeaderInt(buf, h.Mode)
	writeHeaderInt(buf, h.Uid)
	writeHeaderInt(buf, h.Gid)
	writeHeaderInt(buf, h.Links)
	if t := h.ModTime; t.IsZero() {
		writeHeaderInt(buf, 0)
	} else {
		writeHeaderInt(buf, t.Unix())
	}
	writeHeaderInt(buf, h.Length)
	writeHeaderInt(buf, h.Major)
	writeHeaderInt(buf, h.Minor)
	writeHeaderInt(buf, h.RMajor)
	writeHeaderInt(buf, h.RMinor)
	writeHeaderInt(buf, z)
	writeHeaderInt(buf, 0)
	writeFilename(buf, h.Filename)

	w.blocks += headerLen + z
	if mod := w.blocks % 4; mod != 0 && !trailing {
		zs := make([]byte, 4-mod)
		n, _ := buf.Write(zs)
		w.blocks += int64(n)
	}

	if _, err := io.Copy(w.inner, buf); err != nil {
		w.err = err
	}
	return w.err
}

type Reader struct {
	inner   *bufio.Reader
	curr    rw.Reader
	err     error
	discard int
}

func NewReader(r io.Reader) *Reader {
	return &Reader{inner: bufio.NewReader(r)}
}

func (r *Reader) Next() (*tape.Header, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.discard > 0 {
		r.inner.Discard(r.discard)
		r.discard = 0
	}
	h, err := r.next()
	if err != nil {
		r.err = err
		return nil, err
	}

	if h.Filename == trailer {
		return nil, io.EOF
	}
	if mod := h.Length % 4; mod > 0 {
		r.discard = int(4 - mod)
	}
	r.curr = rw.NewReader(r.inner, int(h.Length))
	return h, nil
}

func (r *Reader) next() (*tape.Header, error) {
	var (
		h tape.Header
		z int64
	)
	if err := readMagic(r.inner); err != nil {
		r.err = err
		return nil, err
	}
	h.Inode = readHeaderField(r.inner)
	h.Mode = readHeaderField(r.inner)
	h.Uid = readHeaderField(r.inner)
	h.Gid = readHeaderField(r.inner)
	h.Links = readHeaderField(r.inner)
	h.ModTime = time.Unix(readHeaderField(r.inner), 0)
	h.Length = readHeaderField(r.inner)
	h.Major = readHeaderField(r.inner)
	h.Minor = readHeaderField(r.inner)
	h.RMajor = readHeaderField(r.inner)
	h.RMinor = readHeaderField(r.inner)
	z = readHeaderField(r.inner)
	h.Check = readHeaderField(r.inner)
	h.Filename = readFilename(r.inner, z)
	if mod := (headerLen + z) % 4; mod != 0 {
		_, err := r.inner.Discard(4 - int(mod))
		if err != nil {
			return nil, err
		}
	}
	return &h, nil
}

func (r *Reader) Read(bs []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	n, err := r.curr.Read(bs)
	if err != nil && err != io.EOF {
		r.err = err
	}
	return n, err
}

func readMagic(r io.Reader) error {
	bs := make([]byte, magicLen)
	if _, err := io.ReadFull(r, bs); err != nil {
		return err
	}
	if bytes.Equal(bs, magicCRC) || bytes.Equal(bs, magicASCII) {
		return nil
	}
	return tape.ErrUnsupported
}

func readFilename(r io.Reader, n int64) string {
	bs := make([]byte, n)
	if _, err := io.ReadFull(r, bs); err != nil {
		return ""
	}
	return string(bs[:n-1])
}

func readHeaderField(r io.Reader) int64 {
	//TODO: check for rewrite with fmt.Fscanf()
	bs := make([]byte, fieldLen)
	if _, err := io.ReadFull(r, bs); err != nil {
		return -1
	}
	i, err := strconv.ParseInt("0x"+string(bs), 0, 64)
	if err != nil {
		return -1
	}
	return i
}

func writeHeaderInt(w *bytes.Buffer, f int64) {
	fmt.Fprintf(w, "%08x", uint64(f))
}

func writeFilename(w *bytes.Buffer, f string) {
	io.WriteString(w, f+"\x00")
}
