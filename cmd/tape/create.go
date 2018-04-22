package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
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
	// recurse := cmd.Flag.Bool("r", false, "recurse")
	preserve := cmd.Flag.Bool("p", false, "preserve")
	compress := cmd.Flag.Bool("c", false, "compress")
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
	var err error
	switch e := filepath.Ext(cmd.Flag.Arg(0)); e {
	case ".cpio":
		err = createCPIO(cmd.Flag.Arg(0), files, *compress, *preserve)
	case ".ar":
		err = createAR(cmd.Flag.Arg(0), files, *compress, *preserve)
	default:
		return ErrNotSupported(e)
	}
	return err
}

func createAR(a string, files []string, compress, preserve bool) error {
	f, err := os.Create(a)
	if err != nil {
		return err
	}
	w, err := ar.NewWriter(f)
	if err != nil {
		return err
	}
	defer func() {
		w.Close()
		f.Close()
	}()
	for _, f := range files {
		f, err := os.Open(f)
		if err != nil {
			return err
		}
		i, err := f.Stat()
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if compress {
			g := gzip.NewWriter(&buf)
			if _, err := io.Copy(g, f); err != nil {
				return err
			}
			if err := g.Close(); err != nil {
				return err
			}
		} else {
			if _, err := io.Copy(&buf, f); err != nil {
				return err
			}
		}
		h := tape.Header{
			Filename: i.Name(),
			ModTime:  i.ModTime(),
			Mode:     int64(i.Mode()),
			Length:   int64(buf.Len()),
		}
		if i, ok := i.Sys().(*syscall.Stat_t); ok && preserve {
			h.Uid, h.Gid = int64(i.Uid), int64(i.Gid)
		} else {
			h.Uid, h.Gid = int64(os.Geteuid()), int64(os.Getgid())
			h.ModTime = time.Now()
		}
		if err := w.WriteHeader(&h); err != nil {
			return err
		}
		if _, err := io.Copy(w, &buf); err != nil {
			return err
		}
	}
	return nil
}

func createCPIO(a string, files []string, compress, preserve bool) error {
	f, err := os.Create(a)
	if err != nil {
		return err
	}
	w := cpio.NewWriter(f)
	defer func() {
		w.Close()
		f.Close()
	}()
	for _, f := range files {
		f, err := os.Open(f)
		if err != nil {
			return err
		}
		i, err := f.Stat()
		if err != nil {
			return err
		}
		var buf bytes.Buffer
		if compress {
			g := gzip.NewWriter(&buf)
			if _, err := io.Copy(g, f); err != nil {
				return err
			}
			if err := g.Close(); err != nil {
				return err
			}
		} else {
			if _, err := io.Copy(&buf, f); err != nil {
				return err
			}
		}
		h := tape.Header{
			Filename: i.Name(),
			ModTime:  i.ModTime(),
			Mode:     int64(i.Mode()),
			Length:   int64(buf.Len()),
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
		if err := w.WriteHeader(&h); err != nil {
			return err
		}
		if _, err := io.Copy(w, &buf); err != nil {
			return err
		}
	}

	return nil
}
