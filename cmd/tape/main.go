package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"text/template"

	"github.com/midbel/cli"
	"github.com/midbel/tape"
)

type ErrNotSupported string

func (e ErrNotSupported) Error() string {
	v := string(e)
	if v[0] == '.' {
		v = v[1:]
	}
	return fmt.Sprintf("tape: unsupported archive type %s", v)
}

type OpenFunc func(io.Reader) (tape.Reader, error)

var commands = []*cli.Command{
	{
		Run:   runCreate,
		Usage: "create [-c] [-p] <archive> <file,...>",
		Alias: []string{"make"},
		Short: "create a new cpio or ar archives",
		Desc:  "",
	},
	{
		Run:   runExtract,
		Usage: "extract [-p] <archive,...>",
		Short: "extract the content of cpio and/or ar archives",
		Desc:  "",
	},
	{
		Run:   runList,
		Usage: "list [-b] <archive,...>",
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
