package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"syscall"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/tape/ar"
	"github.com/midbel/tape/cpio"
)

func runExtract(cmd *cli.Command, args []string) error {
	preserve := cmd.Flag.Bool("p", false, "preserve")
	datadir := cmd.Flag.String("d", os.TempDir(), "datadir")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	for _, a := range cmd.Flag.Args() {
		var err error
		switch e := filepath.Ext(cmd.Flag.Arg(0)); e {
		case ".cpio":
			err = extractCPIO(a, *datadir, *preserve)
		case ".ar":
			err = extractAR(a, *datadir, *preserve)
		default:
			return ErrNotSupported(e)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func extractAR(file, datadir string, preserve bool) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	a, err := ar.NewReader(f)
	if err != nil {
		return err
	}
	for {
		h, err := a.Next()
		switch err {
		case io.EOF:
			return nil
		case nil:
		default:
			return err
		}
		var buf bytes.Buffer
		if _, err := io.CopyN(&buf, a, h.Length); err != nil {
			return err
		}
		r := bufio.NewReader(&buf)
		if g, err := gzip.NewReader(r); err == nil {
			bs := make([]byte, h.Length)
			if _, err := io.ReadFull(g, bs); err != nil {
				return err
			}
			buf.Write(bs)
		} else {
			r.Reset(&buf)
		}
		p := filepath.Join(datadir, h.Filename)
		if err := ioutil.WriteFile(p, buf.Bytes(), os.FileMode(h.Mode)); err != nil {
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
	return nil
}

func extractCPIO(file, datadir string, preserve bool) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	c := cpio.NewReader(f)
	for {
		h, err := c.Next()
		switch err {
		case io.EOF:
			return nil
		case nil:
		default:
			return err
		}
		var buf bytes.Buffer
		if _, err := io.CopyN(&buf, c, h.Length); err != nil {
			return err
		}
		r := bufio.NewReader(&buf)
		if g, err := gzip.NewReader(r); err == nil {
			bs := make([]byte, h.Length)
			if _, err := io.ReadFull(g, bs); err != nil {
				return err
			}
			buf.Write(bs)
		} else {
			r.Reset(&buf)
		}
		p := filepath.Join(datadir, h.Filename)
		dir, _ := filepath.Split(h.Filename)
		if err := os.MkdirAll(dir, 0755); dir != "" && err != nil && !os.IsExist(err) {
			return err
		}
		if err := ioutil.WriteFile(p, buf.Bytes(), os.FileMode(h.Mode)); err != nil {
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
	return nil
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
