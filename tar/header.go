package tar

import (
	"io/fs"
	"os"
	"os/user"
	"strconv"
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

func (t TypeFlag) isRegular() bool {
	return t == TypeReg
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
	ustarver   = "00"
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

	paxDir = "PaxHeaders"
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

func FileInfoHeader(file string) (*Header, error) {
	s, err := os.Stat(file)
	if err != nil {
		return nil, err
	}
	var k TypeFlag
	if s.IsDir() {
		k = TypeDir
	} else if b := s.Mode() & fs.ModeSymlink; b == fs.ModeSymlink {
		k = TypeSymLink
	} else {
		k = TypeReg
	}
	h := Header{
		Name:       file,
		Size:       s.Size(),
		ModTime:    s.ModTime(),
		Perm:       int64(s.Mode()),
		Type:       k,
		Uid:        os.Getuid(),
		Gid:        os.Getgid(),
		PaxHeaders: make(map[string]string),
	}
	uid := strconv.Itoa(h.Uid)
	if u, err := user.LookupId(uid); err == nil {
		h.User = u.Username
	}
	gid := strconv.Itoa(h.Gid)
	if g, err := user.LookupGroupId(gid); err == nil {
		h.Group = g.Name
	}
	if h.Type == TypeSymLink {

	}
	return &h, nil
}

func (h *Header) merge(other *Header) {
	if other == nil {
		return
	}
	for k, v := range other.PaxHeaders {
		h.PaxHeaders[k] = v
		switch k {
		case paxAtime:
			h.AccessTime = other.AccessTime
		case paxMtime:
			h.ModTime = other.ModTime
		case paxPath:
			h.Name = other.Name
		case paxLink:
			h.LinkName = other.LinkName
		case paxUser:
			h.User = other.User
		case paxGroup:
			h.Group = other.Group
		case paxSize:
			h.Size = other.Size
		case paxUid:
			h.Uid = other.Uid
		case paxGid:
			h.Gid = other.Gid
		}
	}
}
