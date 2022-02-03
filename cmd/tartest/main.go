package main

import (
	"compress/gzip"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/midbel/tape/tar"
)

func main() {
	apk := flag.Bool("a", false, "apk archive")
	flag.Parse()

	r, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer r.Close()

	var read func(io.Reader) error = readBasic
	if *apk {
		read = readAPK
	}
	if err := read(r); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func readAPK(r io.Reader) error {
	z, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer z.Close()
	return readBasic(z)
}

func readBasic(r io.Reader) error {
	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return err
		}
		fmt.Printf("%+s (%c) %s -> %d\n", h.Name, h.Type, h.ModTime, h.Size)
    io.Copy(io.Discard, tr)
	}
	return nil
}
