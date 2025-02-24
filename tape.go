package tape

import (
	"errors"
	"io"
	"os"
	"os/user"
	"strconv"
	"time"
)

var (
	ErrMagic       = errors.New("tape: invalid Magic")
	ErrRead        = errors.New("tape: invalid read")
	ErrTooShort    = errors.New("tape: write too short")
	ErrTooLong     = errors.New("tape: write too long")
	ErrUnsupported = errors.New("tape: unsupported format")
	ErrHeader      = errors.New("tape: invalid header")
	ErrClosed      = errors.New("tape: archive closed")
)

type Reader interface {
	io.Reader
	Next() (*Header, error)
}

type Writer interface {
	io.WriteCloser
	WriteHeader(*Header) error
}

func Convert(r Reader, w Writer) error {
	for {
		h, err := r.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		if err := w.WriteHeader(h); err != nil {
			return err
		}
		if _, err := io.CopyN(w, r, h.Size); err != nil {
			return err
		}
	}
	return nil
}

type Header struct {
	Inode    int64
	Mode     int64
	Uid      int64
	Gid      int64
	Links    int64
	Size     int64
	Major    int64
	Minor    int64
	RMajor   int64
	RMinor   int64
	Check    int64
	ModTime  time.Time
	Filename string
}

func FileInfoHeaderFromFile(file *os.File) (*Header, error) {
	i, err := file.Stat()
	if err != nil {
		return nil, err
	}	
	h := Header{
		Filename: file.Name(),
		Size:     i.Size(),
		Mode:     int64(i.Mode()),
		Uid:      int64(os.Getuid()),
		Gid:      int64(os.Getgid()),
		ModTime:  i.ModTime(),
	}
	return &h, nil
}

func FileInfoHeader(file string) (*Header, error) {
	r, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return FileInfoHeaderFromFile(r)
}

func (h Header) User() string {
	var (
		id     = strconv.FormatInt(h.Uid, 10)
		u, err = user.LookupId(id)
	)
	if err != nil {
		return id
	}
	return u.Username
}

func (h Header) Group() string {
	var (
		id     = strconv.FormatInt(h.Gid, 10)
		g, err = user.LookupGroupId(id)
	)
	if err != nil {
		return id
	}
	return g.Name
}


type limitedWriter struct {
	W io.Writer
	N int64
	r int64
}

func LimitWriter(w io.Writer, n int64) io.Writer {
	return &limitedWriter{
		W: w,
		N: n,
	}
}

func (w *limitedWriter) Write(b []byte) (int, error) {
	if w.N <= 0 {
		return 0, ErrTooLong
	}
	var (
		remain int64
		err    error
	)
	if remain = w.N - int64(len(b)); remain < 0 {
		b = b[:w.N]
		err = ErrTooLong
	}
	n, err1 := w.W.Write(b)
	if err == nil {
		err = err1
	}
	w.N -= int64(n)
	return n, err
}

func (w *limitedWriter) Available() int64 {
	return w.N
}