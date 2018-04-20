package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"syscall"

	"github.com/midbel/tape/cpio"
)

func main() {
	recurse := flag.Bool("r", false, "recurse")
	quiet := flag.Bool("q", false, "quiet")
	flag.Parse()

	var err error
	switch flag.NArg() {
	case 0:
		os.Exit(1)
	case 1:
		err = readFrom(flag.Arg(0), *quiet)
	default:
		args := flag.Args()
		err = writeTo(args[0], args[1:], *recurse)
	}
	if err != nil {
		log.Fatalln(err)
	}
}

func writeTo(file string, fs []string, recurse bool) error {
	f, err := os.Create(file)
	if err != nil {
		return err
	}

	w := cpio.NewWriter(f)
	defer func() {
		log.Println("close", file)
		w.Close()
		f.Close()
	}()
	for _, f := range fs {
		if err := appendFile(w, f, recurse); err != nil {
			return err
		}
	}
	return nil
}

func appendFile(w *cpio.Writer, f string, r bool) error {
	log.Println("process", f)
	i, err := os.Stat(f)
	if err != nil {
		return err
	}
	if i.IsDir() && r {
		infos, err := ioutil.ReadDir(f)
		if err != nil {
			return err
		}
		for _, i := range infos {
			if err := appendFile(w, filepath.Join(f, i.Name()), r); err != nil {
				return err
			}
		}
		return nil
	}
	stat, ok := i.Sys().(*syscall.Stat_t)
	if !ok || stat == nil {
		return fmt.Errorf("can not get stat for info %s", f)
	}
	h := cpio.Header{
		Filename: f,
		Mode:     int64(i.Mode()),
		Length:   i.Size(),
		ModTime:  i.ModTime(),
		Uid:      int64(stat.Uid),
		Gid:      int64(stat.Gid),
		Inode:    int64(stat.Ino),
		Links:    int64(stat.Nlink),
		Major:    int64(stat.Dev >> 32),
		Minor:    int64(stat.Dev & 0xFFFFFFFF),
	}
	if err := w.WriteHeader(&h); err != nil {
		return err
	}
	bs, err := ioutil.ReadFile(f)
	if err != nil {
		return err
	}
	_, err = w.Write(bs)
	return err
}

func readFrom(file string, quiet bool) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	r := cpio.NewReader(f)
	w := ioutil.Discard
	if !quiet {
		w = os.Stderr
	}
	for {
		h, err := r.Next()
		if err == io.EOF {
			log.Printf("%+v", h)
			break
		}
		if err != nil {
			return err
		}
		log.Printf("%+v", h)
		if _, err = io.CopyN(w, r, h.Length); err != nil && err != io.EOF {
			return err
		}
	}
	return nil
}
