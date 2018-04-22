package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
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
		Usage: "create [-z] <archive> <file,...>",
		Short: "",
		Desc:  "",
	},
	{
		Run:   runExtract,
		Usage: "extract <archive>",
		Short: "",
		Desc:  "",
	},
	{
		Run:   runList,
		Usage: "list [-v] <archive>",
		Alias: []string{"ls"},
		Short: "",
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
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var err error
	switch e := filepath.Ext(cmd.Flag.Arg(0)); e {
	case ".cpio":
		err = createCPIO()
	case ".ar":
		err = createAR()
	default:
		return fmt.Errorf("tape: can not create %s archive", e)
	}
	return err
}

func runExtract(cmd *cli.Command, args []string) error {
	if err := cmd.Flag.Parse(args); err != nil {
		return err
	}
	var err error
	switch e := filepath.Ext(cmd.Flag.Arg(0)); e {
	case ".cpio":
		err = extractCPIO()
	case ".ar":
		err = extractAR()
	default:
		return fmt.Errorf("tape: can not extract %s archive", e)
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

func createAR() error   { return nil }
func createCPIO() error { return nil }

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
	_ = c
	return nil
}

func extractAR() error   { return nil }
func extractCPIO() error { return nil }

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
