package tape

import (
	"os/user"
	"strconv"
	"time"
)

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
