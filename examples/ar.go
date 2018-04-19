package main

import (
	"flag"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/midbel/mack/ar"
)

func main() {
	empty := flag.Bool("e", false, "")
	quiet := flag.Bool("q", false, "")
	list := flag.Bool("l", false, "")
	flag.Parse()
	switch flag.NArg() {
	case 1:
		if *list {
			readList(flag.Arg(0))
		} else {
			readFrom(flag.Arg(0))
		}
	case 2:
		writeTo(flag.Arg(0), flag.Arg(1), *empty)
		if !*quiet {
			readFrom(flag.Arg(0))
		}
	default:
		return
	}
}

func writeTo(a, p string, e bool) {
	f, err := os.Create(a)
	if err != nil {
		log.Fatalln(err)
	}

	w, err := ar.NewWriter(f)
	if err != nil {
		log.Fatalln(err)
	}
	defer func() {
		if err := w.Close(); err != nil {
			log.Fatalln(err)
		}
		f.Close()
	}()
	err = filepath.Walk(p, func(p string, i os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if i.IsDir() {
			return nil
		}
		h := ar.Header{
			Name:    i.Name(),
			ModTime: i.ModTime(),
			Uid:     1000,
			Gid:     1000,
			Mode:    int(i.Mode()),
			Length:  int(i.Size()),
		}
		if e {
			h.ModTime = time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC)
			h.Uid, h.Gid = 0, 0
		}
		if err := w.WriteHeader(&h); err != nil {
			return err
		}
		r, err := os.Open(p)
		if err != nil {
			return err
		}
		defer r.Close()
		_, err = io.Copy(w, r)
		if err != nil {
			log.Println(err)
		}
		return err
	})
	if err != nil {
		log.Fatalln(err)
	}
}

func readList(p string) {
	hs, err := ar.List(p)
	if err != nil {
		log.Fatalln(err)
	}
	for _, h := range hs {
		log.Printf("%+v", h)
	}
}

func readFrom(p string) {
	f, err := os.Open(p)
	if err != nil {
		log.Fatalln(err)
	}
	defer f.Close()

	r, err := ar.NewReader(f)
	if err != nil {
		log.Fatalln(err)
	}
	for {
		h, err := r.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalln("get header", err)
		}
		bs := make([]byte, h.Length)
		if n, err := io.ReadFull(r, bs); err != nil {
			log.Fatalf("get body: %s: %d - %d", err, n, h.Length)
		}
		log.Printf("%+v =>\n%s", h, string(bs))
	}
}
