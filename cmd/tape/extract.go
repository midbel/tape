package main

import (
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"syscall"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/tape"
	"github.com/midbel/tape/ar"
	"github.com/midbel/tape/cpio"
)

func runExtract(cmd *cli.Command, args []string) error {
	preserve := cmd.Flag.Bool("p", false, "preserve")
	datadir := cmd.Flag.String("d", os.TempDir(), "datadir")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	f, err := os.Open(cmd.Flag.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()

	var r tape.Reader
	switch e := filepath.Ext(f.Name()); e {
	case ".cpio":
		r = cpio.NewReader(f)
	case ".ar":
		if a, e := ar.NewReader(f); e != nil {
			err = e
		} else {
			r = a
		}
	default:
		return ErrNotSupported(e)
	}
	if err != nil {
		return err
	}
	ms := cmd.Flag.Args()
	return extractArchive(r, *datadir, ms[1:], *preserve)
}

func extractArchive(r tape.Reader, datadir string, members []string, preserve bool) error {
	ms := sort.StringSlice(members)
	ms.Sort()
	for {
		h, err := r.Next()
		switch err {
		case nil:
		case io.EOF:
			return nil
		default:
			return err
		}
		ix := ms.Search(h.Filename)
		if ms.Len() > 0 && (ix >= ms.Len() || ms[ix] != h.Filename) {
			_, err := io.CopyN(ioutil.Discard, r, h.Length)
			if err != nil {
				return err
			}
			continue
		}
		p := filepath.Join(datadir, h.Filename)
		w, err := os.Create(p)
		if err != nil {
			return err
		}
		if _, err := io.CopyN(w, r, h.Length); err != nil {
			return err
		}
		if !preserve {
			h.Uid, h.Gid = int64(os.Geteuid()), int64(os.Getgid())
			h.ModTime = time.Now()
		}
		if err := updateFileInfo(p, h.Uid, h.Gid, h.ModTime.Unix()); err != nil {
			return err
		}
	}
}

func updateFileInfo(p string, uid, gid, mod int64) error {
	if err := syscall.Chown(p, int(uid), int(gid)); err != nil {
		return err
	}
	t := syscall.Utimbuf{
		Actime:  mod,
		Modtime: mod,
	}
	if err := syscall.Utime(p, &t); err != nil {
		return err
	}
	return nil
}
