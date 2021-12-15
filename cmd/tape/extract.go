package main

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/tape"
	"github.com/midbel/tape/ar"
	"github.com/midbel/tape/cpio"
)

func runExtract(cmd *cli.Command, args []string) error {
	var (
		preserve = cmd.Flag.Bool("p", false, "preserve")
		datadir  = cmd.Flag.String("d", os.TempDir(), "datadir")
	)
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
	args = cmd.Flag.Args()
	return extractArchive(r, *datadir, args[1:], *preserve)
}

func extractArchive(r tape.Reader, datadir string, members []string, preserve bool) error {
	if len(members) == 0 {
		return nil
	}
	sort.Strings(members)
	for {
		err := extractFile(r, datadir, members, preserve)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
	}
	return nil
}

func extractFile(r tape.Reader, datadir string, members []string, preserve bool) error {
	h, err := r.Next()
	if err != nil {
		return err
	}
	ix := sort.SearchStrings(members, h.Filename)
	if ix >= len(members) || members[ix] != h.Filename {
		_, err = io.CopyN(io.Discard, r, h.Length)
		return err
	}

	file := filepath.Join(datadir, h.Filename)
	w, err := os.Create(file)
	if err != nil {
		return err
	}
	defer w.Close()

	if _, err := io.CopyN(w, r, h.Length); err != nil {
		return err
	}

	if !preserve {
		h.Uid, h.Gid = int64(os.Geteuid()), int64(os.Getgid())
		h.ModTime = time.Now()
	}
	return updateFileInfo(file, h.Uid, h.Gid, h.ModTime)
}

func updateFileInfo(file string, uid, gid int64, mod time.Time) error {
	if err := os.Chown(file, int(uid), int(gid)); err != nil {
		return err
	}
	return os.Chtimes(file, mod, mod)
}
