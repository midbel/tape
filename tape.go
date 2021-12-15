package tape

import (
	"errors"
	"io"
	"os/user"
	"strconv"
	"time"
)

var (
	ErrMagic       = errors.New("tape: Invalid Magic")
	ErrTooShort    = errors.New("tape: write too short")
	ErrTooLong     = errors.New("tape: write too long")
	ErrUnsupported = errors.New("tape: unsupported format")
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
		if _, err := io.CopyN(w, r, h.Length); err != nil {
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
	Length   int64
	Major    int64
	Minor    int64
	RMajor   int64
	RMinor   int64
	Check    int64
	ModTime  time.Time
	Filename string
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
