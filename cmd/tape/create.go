package main

import (
	"bufio"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/tape"
	"github.com/midbel/tape/ar"
	"github.com/midbel/tape/cpio"
)

func runCreate(cmd *cli.Command, args []string) error {
	preserve := cmd.Flag.Bool("p", false, "preserve")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var files []string
	if cmd.Flag.NArg() == 1 {
		s := bufio.NewScanner(os.Stdin)
		for s.Scan() {
			files = append(files, s.Text())
		}
		if err := s.Err(); err != nil {
			return err
		}
	} else {
		args := cmd.Flag.Args()
		files = args[1:]
	}
	f, err := os.Create(cmd.Flag.Arg(0))
	if err != nil {
		return err
	}
	defer f.Close()

	var w tape.Writer
	switch e := filepath.Ext(f.Name()); e {
	case ".cpio":
		w = cpio.NewWriter(f)
	case ".ar":
		a, err := ar.NewWriter(f)
		if err != nil {
			return err
		}
		w = a
	default:
		return ErrNotSupported(e)
	}
	return createArchive(w, files, *preserve)
}

func createArchive(w tape.Writer, files []string, preserve bool) error {
	defer w.Close()
	for _, f := range files {
		if err := appendFile(w, f, preserve); err != nil {
			return err
		}
	}
	return nil
}

func appendFile(w tape.Writer, file string, preserve bool) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()

	i, err := r.Stat()
	if err != nil {
		return err
	}
	h := tape.Header{
		Filename: i.Name(),
		ModTime:  i.ModTime(),
		Mode:     int64(i.Mode()),
		Length:   i.Size(),
	}
	if i, ok := i.Sys().(*syscall.Stat_t); ok {
		h.Uid = int64(i.Uid)
		h.Gid = int64(i.Gid)
		h.Major = int64(i.Dev >> 32)
		h.Minor = int64(i.Dev & 0xFFFFFFFF)
		h.Links = int64(i.Nlink)
		h.Inode = int64(i.Ino)
	}
	if !preserve {
		h.Uid, h.Gid = int64(os.Geteuid()), int64(os.Getgid())
		h.ModTime = time.Now()
	}
	if err = w.WriteHeader(&h); err != nil {
		return err
	}
	_, err = io.Copy(w, r)
	return err
}
