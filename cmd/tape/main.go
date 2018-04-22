package main

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"text/template"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/tape/ar"
	"github.com/midbel/tape/cpio"
)

var commands = []*cli.Command{
	{
		Run:   runCreate,
		Usage: "create [-c] <archive> <file,...>",
		Alias: []string{"make"},
		Short: "create a new cpio or ar archives",
		Desc:  "",
	},
	{
		Run:   runExtract,
		Usage: "extract <archive,...>",
		Short: "extract the content of cpio and/or ar archives",
		Desc:  "",
	},
	{
		Run:   runList,
		Usage: "list [-b] [-v] <archive,...>",
		Alias: []string{"ls"},
		Short: "list the content of cpio and/or ar archives",
		Desc:  "",
	},
}

const helpText = `{{.Name}} create or extract file(s) from cpio or ar archives.

Usage:

  {{.Name}} command [arguments]

The commands are:

{{range .Commands}}{{printf "  %-9s %s" .String .Short}}
{{end}}

Use {{.Name}} [command] -h for more information about its usage.
`

func main() {
	log.SetFlags(0)
	if err := cli.Run(commands, usage, nil); err != nil {
		log.Fatalln(err)
	}
}

func usage() {
	data := struct {
		Name     string
		Commands []*cli.Command
	}{
		Name:     filepath.Base(os.Args[0]),
		Commands: commands,
	}
	t := template.Must(template.New("help").Parse(helpText))
	t.Execute(os.Stderr, data)

	os.Exit(2)
}

func runCreate(cmd *cli.Command, args []string) error {
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
		err = createCPIO(cmd.Flag.Arg(0), files, *compress)
	case ".ar":
		err = createAR(cmd.Flag.Arg(0), files, *compress)
	default:
		return fmt.Errorf("tape: can not create %s archive", e)
	}
	return err
}

func runExtract(cmd *cli.Command, args []string) error {
	datadir := cmd.Flag.String("d", os.TempDir(), "datadir")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var err error
	for _, a := range cmd.Flag.Args() {
		switch e := filepath.Ext(cmd.Flag.Arg(0)); e {
		case ".cpio":
			err = extractCPIO(a, *datadir)
		case ".ar":
			err = extractAR(a, *datadir)
		default:
			return fmt.Errorf("tape: can not extract %s archive", e)
		}
		if err != nil {
			break
		}
	}
	return err
}

func runList(cmd *cli.Command, args []string) error {
	verbose := cmd.Flag.Bool("v", false, "verbose")
	block := cmd.Flag.String("b", "", "block size")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var err error
	for i, a := range cmd.Flag.Args() {
		switch e := filepath.Ext(a); e {
		case ".cpio":
			err = listCPIO(a, *block, *verbose)
		case ".ar":
			err = listAR(a, *block, *verbose)
		default:
			return fmt.Errorf("tape: can not list %s archive", e)
		}
		if err != nil {
			break
		}
		if i < cmd.Flag.NArg()-1 {
			log.Println()
		}
	}
	return err
}

func createAR(a string, files []string, compress bool) error {
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
		h := ar.Header{
			Filename: i.Name(),
			ModTime:  i.ModTime(),
			Mode:     int64(i.Mode()),
			Length:   int64(buf.Len()),
		}
		if i, ok := i.Sys().(*syscall.Stat_t); ok {
			h.Uid, h.Gid = int64(i.Uid), int64(i.Gid)
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

func createCPIO(a string, files []string, compress bool) error {
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
		h := cpio.Header{
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
		if err := w.WriteHeader(&h); err != nil {
			return err
		}
		if _, err := io.Copy(w, &buf); err != nil {
			return err
		}
	}

	return nil
}

func listAR(file, block string, verbose bool) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	a, err := ar.NewReader(f)
	if err != nil {
		return err
	}
	logger := log.New(os.Stdout, "", 0)
	logger.Println(file + ":")
	if verbose {
		w := tabwriter.NewWriter(os.Stdout, 6, 2, 2, ' ', 0)
		defer w.Flush()

		logger.SetOutput(w)
	}
	var coeff int64
	switch block {
	default:
		coeff = 1
	case "K", "k":
		coeff = 1024
	case "M", "m":
		coeff = 1024 * 1024
	case "G", "g":
		coeff = 1024 * 1024 * 1024
	}
	for {
		h, err := a.Next()
		switch err {
		case nil:
		case io.EOF:
			return nil
		default:
			return err
		}
		if !verbose {
			logger.Println(h.Filename)
		} else {
			h.Length /= coeff
			printHeaderAR(logger, h)
		}
		if _, err := io.CopyN(ioutil.Discard, a, h.Length); err != nil {
			return err
		}
	}
	return nil
}

func listCPIO(file, block string, verbose bool) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	c := cpio.NewReader(f)

	logger := log.New(os.Stdout, "", 0)
	logger.Println(file + ":")
	if verbose {
		w := tabwriter.NewWriter(os.Stdout, 6, 2, 2, ' ', 0)
		defer w.Flush()

		logger.SetOutput(w)
	}
	var coeff int64
	switch block {
	default:
		coeff = 1
	case "K", "k":
		coeff = 1024
	case "M", "m":
		coeff = 1024 * 1024
	case "G", "g":
		coeff = 1024 * 1024 * 1024
	}
	for {
		h, err := c.Next()
		switch err {
		case nil:
		case io.EOF:
			return nil
		default:
			return err
		}
		if !verbose {
			logger.Println(h.Filename)
		} else {
			h.Length /= coeff
			printHeaderCPIO(logger, h)
		}
		if _, err := io.CopyN(ioutil.Discard, c, h.Length); err != nil {
			return err
		}
	}
	return nil
}

func extractAR(file, datadir string) error {
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
			var other bytes.Buffer
			io.Copy(&other, g)
			io.Copy(&buf, &other)
		} else {
			r.Reset(&buf)
		}
		p := filepath.Join(datadir, h.Filename)
		if err := ioutil.WriteFile(p, buf.Bytes(), os.FileMode(h.Mode)); err != nil {
			return err
		}
	}
	return nil
}
func extractCPIO(file, datadir string) error {
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
			var other bytes.Buffer
			io.Copy(&other, g)
			io.Copy(&buf, &other)
		} else {
			r.Reset(&buf)
		}
		p := filepath.Join(datadir, h.Filename)
		dir, _ := filepath.Split(h.Filename)
		if dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil && !os.IsExist(err) {
				return err
			}
		}
		if err := ioutil.WriteFile(p, buf.Bytes(), os.FileMode(h.Mode)); err != nil {
			return err
		}
	}
	return nil
}

func printHeaderCPIO(w *log.Logger, h *cpio.Header) {
	n := time.Now()
	var f string
	if n.Year() == h.ModTime.Year() {
		f = "Jan 02 15:05"
	} else {
		f = "Jan 02  2006"
	}
	m := strings.Join(parseMode(h.Mode), "")
	w.Printf("%s\t%s\t%s\t%d\t%s\t%s", m, h.User(), h.Group(), h.Length, h.ModTime.Format(f), h.Filename)
}

func printHeaderAR(w *log.Logger, h *ar.Header) {
	n := time.Now()
	var f string
	if n.Year() == h.ModTime.Year() {
		f = "Jan 02 15:05"
	} else {
		f = "Jan 02  2006"
	}
	m := strings.Join(parseMode(h.Mode), "")
	w.Printf("%s\t%s\t%s\t%d\t%s\t%s", m, h.User(), h.Group(), h.Length, h.ModTime.Format(f), h.Filename)
}

func parseMode(i int64) []string {
	var r, w, x int64 = 0x4, 0x2, 0x1
	vs := make([]string, 3)
	for j := len(vs) - 1; j >= 0; j-- {
		m := i & 0x7
		ms := make([]string, 3)
		for i := range ms {
			ms[i] = "-"
		}
		if m&r == r {
			ms[0] = "r"
		}
		if m&w == w {
			ms[1] = "w"
		}
		if m&x == x {
			ms[2] = "x"
		}
		vs[j] = strings.Join(ms, "")
		i = i >> 3
	}
	return vs
}
