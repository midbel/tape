package tar

import (
	"bytes"
	"io"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/rw"
)

type Writer struct {
	inner io.Writer
	curr  io.Writer
	err   error

	size    int
	written int
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{
		inner: w,
	}
}

func (w *Writer) Write(b []byte) (int, error) {
	if w.err != nil {
		return 0, w.err
	}
	if w.curr == nil {
		return 0, nil
	}
	n, err := w.curr.Write(b)
	w.written += n
	return n, err
}

func (w *Writer) WriteHeader(h *Header) error {
	if w.err != nil {
		return w.err
	}
	if w.err = w.Flush(); w.err != nil {
		return w.err
	}
	if len(h.PaxHeaders) > 0 {
		w.err = w.writePaxHeaders(h)
		if w.err != nil {
			return w.err
		}
	}
	w.err = w.writeHeader(h)
	if w.err == nil {
		w.reset()
		if h.Type == TypeReg {
			w.curr = rw.LimitWriter(w.inner, h.Size)
		}
		w.size = int(h.Size)
	}
	return w.err
}

func (w *Writer) Flush() error {
	defer w.reset()

	if w.err != nil {
		return w.err
	}
	if w.written == 0 {
		return nil
	}
	if w.written != w.size {

	}
	w.pad(w.size)
	return w.err
}

func (w *Writer) Close() error {
	if err := w.Flush(); err != nil {
		return err
	}
	if w.err != nil {
		return w.err
	}
	for i := 0; i < 2; i++ {
		_, w.err = w.inner.Write(zeros)
		if w.err != nil {
			break
		}
	}
	return w.err
}

func (w *Writer) reset() {
	w.size = 0
	w.written = 0
	w.curr = nil
}

func (w *Writer) pad(size int) {
	var pad int
	if mod := size % blockSize; mod > 0 {
		pad = blockSize - mod
	}
	if pad == 0 {
		return
	}
	b := make([]byte, pad)
	_, w.err = w.inner.Write(b)
}

func (w *Writer) writeHeader(h *Header) error {
	var (
		buf = make([]byte, blockSize)
		dir string
		off int
		sum int
	)
	if len(h.Name) > lenName {
		dir, h.Name = filepath.Split(h.Name)
	}
	off = writeString(buf, h.Name, off, lenName)
	off = writeOctal(buf, h.Perm, off, lenMode)
	off = writeOctal(buf, int64(h.Uid), off, lenUid)
	off = writeOctal(buf, int64(h.Gid), off, lenGid)
	off = writeOctal(buf, h.Size, off, lenSize)
	off = writeTime(buf, h.ModTime, off, lenTime)
	sum = off
	off = writeString(buf, emptySum, off, lenSum)
	off = writeType(buf, h.Type, off, lenType)
	off = writeString(buf, h.LinkName, off, lenLink)

	if h.Type.isExtended() {
		off = writeString(buf, ustar, off, lenUstar)
		off = writeString(buf, ustarver, off, lenUstarVersion)
	} else {
		off = writeString(buf, ustar+"  ", off, lenUstar+lenUstarVersion)
		off = writeString(buf, h.User, off, lenUser)
		off = writeString(buf, h.Group, off, lenGroup)
		if h.DevMinor == 0 {
			off += lenDevMinor
		} else {
			off = writeOctal(buf, h.DevMinor, off, lenDevMinor)
		}
		if h.DevMajor == 0 {
			off += lenDevMajor
		} else {
			off = writeOctal(buf, h.DevMajor, off, lenDevMajor)
		}
		off = writeString(buf, dir, off, lenPrefix)
	}
	writeChecksum(buf, sum)

	_, err := w.inner.Write(buf)
	return err
}

func (w *Writer) writePaxHeaders(h *Header) error {
	var (
		buf  bytes.Buffer
		pad  = 3
		keys []string
	)
	for k, v := range h.PaxHeaders {
		if v == "" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		var (
			val  = h.PaxHeaders[key]
			size = len(key) + len(val) + pad
		)
		size += len(strconv.Itoa(size))

		str := strconv.Itoa(size) + " " + key + "=" + val + "\n"
		if z := len(str); size != z {
			size = len(str)
			str = strconv.Itoa(size) + " " + key + "=" + val + "\n"
		}
		buf.WriteString(str)
	}

	x := *h
	dir, file := filepath.Split(h.Name)
	x.Name = "./" + filepath.Join(dir, paxDir, file)
	x.Type = TypeSingleEx
	x.Size = int64(buf.Len())
	x.Uid = 0
	x.Gid = 0
	x.User = ""
	x.Group = ""
	if w.err = w.writeHeader(&x); w.err != nil {
		return w.err
	}
	var n int64
	n, w.err = io.Copy(w.inner, &buf)
	w.pad(int(n))
	return w.err
}

var emptySum = strings.Repeat(" ", lenSum)

func writeChecksum(buf []byte, offset int) {
	var sum int64
	for i := range buf {
		sum += int64(buf[i])
	}
	str := strconv.FormatInt(sum, 8)
	if diff := lenSum - len(str) - 2; diff > 0 {
		str = strings.Repeat("0", diff) + str + "\x00"
	}
	copy(buf[offset:], str)
}

func writeTime(buf []byte, when time.Time, offset, size int) int {
	w := when.Unix()
	return writeOctal(buf, w, offset, size)
}

func writeOctal(buf []byte, oct int64, offset, size int) int {
	str := strconv.FormatInt(oct, 8)
	if diff := size - len(str); diff-1 > 0 {
		str = strings.Repeat("0", diff-1) + str
	}
	return writeString(buf, str, offset, size)
}

func writeType(buf []byte, t TypeFlag, offset, size int) int {
	buf[offset] = byte(t)
	return offset + size
}

func writeBytes(buf, str []byte, offset, size int) int {
	copy(buf[offset:offset+size], str)
	return offset + size
}

func writeString(buf []byte, str string, offset, size int) int {
	return writeBytes(buf, []byte(str), offset, size)
}
