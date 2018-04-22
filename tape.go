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
	i := strconv.FormatInt(h.Uid, 10)
	u, err := user.LookupId(i)
	if err != nil {
		return i
	}
	return u.Username
}

func (h Header) Group() string {
	i := strconv.FormatInt(h.Gid, 10)
	g, err := user.LookupGroupId(i)
	if err != nil {
		return i
	}
	return g.Name
}
