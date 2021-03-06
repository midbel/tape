package main

import (
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/midbel/cli"
	"github.com/midbel/tape"
	"github.com/midbel/tape/ar"
	"github.com/midbel/tape/cpio"
)

const pattern = "%s\t%s\t%s\t%d\t%s\t%s"

const (
	dateYear  = "Jan 02  2006"
	dateTime  = "Jan 02 15:05"
	isoFormat = "2006-01-02T15:05:04"
)

func runList(cmd *cli.Command, args []string) error {
	block := cmd.Flag.String("b", "", "block")
	iso := cmd.Flag.Bool("i", false, "iso format")
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}

	var open OpenFunc
	switch e := filepath.Ext(cmd.Flag.Arg(0)); e {
	case ".cpio":
		open = func(r io.Reader) (tape.Reader, error) {
			return cpio.NewReader(r), nil
		}
	case ".ar":
		open = func(r io.Reader) (tape.Reader, error) {
			return ar.NewReader(r)
		}
	default:
		return ErrNotSupported(e)
	}
	hs, err := listHeaders(cmd.Flag.Arg(0), open)
	if err != nil {
		return err
	}
	sort.Slice(hs, func(i, j int) bool {
		if !*iso {
			return hs[i].Filename < hs[j].Filename
		}
		return hs[i].ModTime.Before(hs[i].ModTime)
	})

	p := Print(*block, *iso)
	defer p.Flush()
	for _, h := range hs {
		p.Print(h)
	}
	return nil
}

func listHeaders(file string, open OpenFunc) ([]*tape.Header, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r, err := open(f)
	if err != nil {
		return nil, err
	}
	var hs []*tape.Header
	for {
		switch h, err := r.Next(); err {
		case nil:
			hs = append(hs, h)
			if _, err := io.CopyN(ioutil.Discard, r, h.Length); err != nil {
				return nil, err
			}
		case io.EOF:
			return hs, nil
		default:
			return nil, err
		}
	}
}

type printer struct {
	*log.Logger
	writer *tabwriter.Writer

	when  time.Time
	coeff int64
}

func Print(block string, iso bool) *printer {
	var p printer

	switch block {
	default:
		p.coeff = 1
	case "K", "k":
		p.coeff = 1024
	case "M", "m":
		p.coeff = 1024 * 1024
	case "G", "g":
		p.coeff = 1024 * 1024 * 1024
	}
	p.writer = tabwriter.NewWriter(os.Stdout, 6, 2, 2, ' ', 0)
	p.Logger = log.New(p.writer, "", 0)
	if !iso {
		p.when = time.Now()
	}

	return &p
}

func (p *printer) Print(h *tape.Header) {
	var f string
	switch {
	case p.when.IsZero():
		f = isoFormat
	case p.when.Year() == h.ModTime.Year():
		f = dateTime
	default:
		f = dateYear
	}
	m := strings.Join(parseMode(h.Mode), "")
	p.Logger.Printf(pattern, m, h.User(), h.Group(), h.Length/p.coeff, h.ModTime.Format(f), h.Filename)
}

func (p *printer) Flush() {
	p.writer.Flush()
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
