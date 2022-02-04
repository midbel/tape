package main

import (
	"compress/gzip"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/midbel/tape/tar"
)

func main() {
	var (
		apk    = flag.Bool("a", false, "apk archive")
		create = flag.Bool("c", false, "create archive")
	)
	flag.Parse()

	if *create {
		args := flag.Args()
		if len(args) <= 1 {
			return
		}
		err := createArchive(args[0], args[1:])
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(12)
		}
		return
	}

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

func createArchive(file string, list []string) error {
	w, err := os.Create(file)
	if err != nil {
		return err
	}
	defer w.Close()

	tw := tar.NewWriter(w)
	defer tw.Close()

	for _, i := range list {
		h, err := tar.FileInfoHeader(i)
		if err != nil {
			return err
		}
		if h.Type == tar.TypeReg {
			h.PaxHeaders["midbel.checksum"] = checksum(i)
		}
		h.PaxHeaders["atime"] = "0" //strconv.FormatInt(now, 10)
		h.PaxHeaders["mtime"] = "0" //strconv.FormatInt(now, 10)
		if err := tw.WriteHeader(h); err != nil {
			fmt.Println("writeHeader", err)
			return err
		}
		if err := copyFile(tw, i); err != nil {
			fmt.Println("copyFile", err)
			return err
		}
	}
	return nil
}

func copyFile(w io.Writer, file string) error {
	r, err := os.Open(file)
	if err != nil {
		return err
	}
	defer r.Close()
	_, err = io.Copy(w, r)
	return err
}

func checksum(file string) string {
	sum := sha1.Sum([]byte(file))
	return hex.EncodeToString(sum[:])
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
