package tar

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/midbel/tape"
)

var ErrHeader = tape.ErrHeader

type TypeFlag byte

const (
	TypeReg      TypeFlag = '0'
	TypeHardLink          = '1'
	TypeSymLink           = '2'
	TypeChar              = '3'
	TypeBlock             = '4'
	TypeDir               = '5'
	TypeSingleEx          = 'x'
	TypeGlobalEx          = 'g'
)

func (t TypeFlag) isExtended() bool {
	return t == TypeSingleEx || t == TypeGlobalEx
}

const (
	blockSize       = 512
	lenName         = 100
	lenMode         = 8
	lenUid          = 8
	lenGid          = 8
	lenSize         = 12
	lenTime         = 12
	lenSum          = 8
	lenType         = 1
	lenLink         = 100
	lenUstar        = 6
	lenUstarVersion = 2
	lenUser         = 32
	lenGroup        = 32
	lenDevMinor     = 8
	lenDevMajor     = 8
	lenPrefix       = 155
)

const (
	ustar      = "ustar"
	paxAtime   = "atime"
	paxMtime   = "mtime"
	paxPath    = "path"
	paxLink    = "linkpath"
	paxUser    = "uname"
	paxGroup   = "gname"
	paxSize    = "size"
	paxUid     = "uid"
	paxGid     = "gid"
	paxCharset = "charset"
)

var (
	zeros = make([]byte, blockSize)
	block = make([]byte, blockSize)
)

type Header struct {
	Type TypeFlag

	Name     string
	LinkName string

	Perm  int64
	Size  int64
	Uid   int
	Gid   int
	User  string
	Group string

	DevMinor int64
	DevMajor int64

	Checksum   []byte
	ModTime    time.Time
	AccessTime time.Time
	ChangeTime time.Time

	PaxHeaders map[string]string
}

func (h *Header) merge(other *Header) {
	if other == nil {
		return
	}
}

type Reader struct {
	inner io.Reader
	curr  io.Reader
	err   error

	read int
}

func NewReader(r io.Reader) *Reader {
	return &Reader{
		inner: r,
	}
}

func (r *Reader) Read(b []byte) (int, error) {
	if r.err != nil {
		return 0, r.err
	}
	if r.curr == nil {
		return 0, tape.ErrRead
	}
	n, err := r.curr.Read(b)
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

func (r *Reader) discard() {
	pad := r.read % blockSize
	if pad == 0 {
		return
	}
	discard(r.inner, int64(blockSize-pad))
}

func (r *Reader) Next() (*Header, error) {
	if r.err != nil {
		return nil, r.err
	}
	if r.curr != nil {
		io.Copy(io.Discard, r.curr)
	}
	hdr, err := r.next()
	if err == nil {
		r.curr = io.LimitReader(r.inner, hdr.Size)
	}
	r.err = err
	return hdr, r.err
}

func (r *Reader) next() (*Header, error) {
	r.read = 0
	var (
		hdr *Header
		pax *Header
		err error
	)
	for {
		hdr, err = r.readHeader()
		if err != nil {
			return nil, err
		}
		if !hdr.Type.isExtended() {
			break
		}
		pax = hdr
	}
	if pax != nil {

	}
	return hdr, err
}

func (r *Reader) readHeader() (*Header, error) {
	if _, err := io.ReadFull(r.inner, block); err != nil {
		return nil, err
	}
	if bytes.Equal(block, zeros) {
		if _, err := io.ReadFull(r.inner, block); err != nil {
			return nil, err
		}
		if bytes.Equal(block, zeros) {
			return nil, io.EOF
		}
		return nil, ErrHeader
	}
	var (
		hdr Header
		off int
		gid int64
		uid int64
		str string
	)
	hdr.PaxHeaders = make(map[string]string)
	hdr.Name, off = readString(block, off, lenName)
	hdr.Perm, off = readOctal(block, off, lenMode)
	uid, off = readOctal(block, off, lenUid)
	hdr.Uid = int(uid)
	gid, off = readOctal(block, off, lenGid)
	hdr.Gid = int(gid)
	hdr.Size, off = readOctal(block, off, lenSize)
	hdr.ModTime, off = readTime(block, off, lenTime)
	hdr.Checksum, off = readBytes(block, off, lenSum)
	hdr.Type, off = readTypeFlag(block, off, lenType)
	hdr.LinkName, off = readString(block, off, lenLink)

	if str, off = readString(block, off, lenUstar); str != ustar {
		return &hdr, nil
	}
	_, off = readString(block, off, lenUstarVersion)
	hdr.User, off = readString(block, off, lenUser)
	hdr.Group, off = readString(block, off, lenGroup)
	hdr.DevMinor, off = readOctal(block, off, lenDevMinor)
	hdr.DevMajor, off = readOctal(block, off, lenDevMajor)
	if str, _ = readString(block, off, lenPrefix); str != "" {
		hdr.Name = filepath.Join(str, hdr.Name)
	}

	if hdr.Type.isExtended() {
		err := r.updateHeader(&hdr)
		if err != nil {
			return nil, err
		}
	}
	return &hdr, nil
}

func (r *Reader) updateHeader(hdr *Header) error {
	scan := bufio.NewScanner(io.LimitReader(r.inner, hdr.Size))
	for scan.Scan() {
		name, value, err := parsePaxRecord(scan.Text())
		if err != nil {
			return err
		}
		fmt.Println(name, value)
    hdr.PaxHeaders[name] = value
		switch name {
		default:
		case paxAtime:
		case paxMtime:
		case paxPath:
		case paxLink:
		case paxUser:
		case paxGroup:
		case paxSize:
		case paxUid:
		case paxGid:
		case paxCharset:
		}
	}
	discard(r.inner, blockSize-hdr.Size)
	return scan.Err()
}

func parsePaxRecord(str string) (string, string, error) {
	size, rest, ok := strings.Cut(str, " ")
	if !ok {
		return "", "", fmt.Errorf("pax header: missing space")
	}
	z, err := strconv.Atoi(size)
	if err != nil {
		return "", "", fmt.Errorf("pax header: invalid integer %s", size)
	}
	if len(str) != z-1 {
		return "", "", fmt.Errorf("pax header: string length mismatched! want %d, got %d", len(str), z)
	}
	name, value, ok := strings.Cut(rest, "=")
	if !ok {
		return "", "", fmt.Errorf("pax header: missing equal")
	}
	return name, value, nil
}

func (r *Reader) skip(z int64) {
	if z == 0 {
		return
	}
	if mod := z % blockSize; mod != 0 {
		z += blockSize - mod
	}
	discard(r.inner, z)
}

func discard(r io.Reader, n int64) {
	io.CopyN(io.Discard, r, n)
}

func readTypeFlag(block []byte, offset, size int) (TypeFlag, int) {
	b := block[offset]
	return TypeFlag(b), offset + size
}

func readBytes(block []byte, offset, size int) ([]byte, int) {
	b := make([]byte, size)
	copy(block[offset:], b)
	return b, offset + size
}

func readString(block []byte, offset, size int) (string, int) {
	var (
		b   = block[offset : offset+size]
		str = strings.Trim(string(b), "\x00")
	)
	return strings.TrimSpace(str), offset + size
}

func readOctal(block []byte, offset, size int) (int64, int) {
	var (
		str, off = readString(block, offset, size)
		oct, _   = strconv.ParseInt(str, 8, 64)
	)
	return oct, off
}

func readTime(block []byte, offset, size int) (time.Time, int) {
	when, off := readOctal(block, offset, size)
	return time.Unix(when, 0).UTC(), off
}

type Writer struct {
	inner io.Writer
}

func NewWriter(w io.Writer) *Writer {
	return &Writer{}
}

func (w *Writer) Write(b []byte) (int, error) {
	return 0, nil
}

func (w *Writer) Flush() error {
	return nil
}

func (w *Writer) Close() error {
	return nil
}
