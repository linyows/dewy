package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"reflect"
	"runtime"
	"strings"

	"github.com/carlescere/scheduler"
	"github.com/hashicorp/logutils"
	flags "github.com/jessevdk/go-flags"
	"github.com/linyows/dewy"
)

const (
	// ExitOK for exit code
	ExitOK int = 0

	// ExitErr for exit code
	ExitErr int = 1
)

type CLI struct {
	outStream, errStream io.Writer
	Command              string
	Config               string `long:"config" short:"c" description:"Path to configuration file"`
	LogLevel             string `long:"log-level" short:"l" arg:"(debug|info|warn|error)" description:"Level displayed as log"`
	Interval             string `long:"interval" short:"i" description:"The polling interval to the repository"`
	Help                 bool   `long:"help" short:"h" description:"show this help message and exit"`
	Version              bool   `long:"version" short:"v" description:"prints the version number"`
}

func (c *CLI) buildHelp(names []string) []string {
	var help []string
	t := reflect.TypeOf(CLI{})

	for _, name := range names {
		f, ok := t.FieldByName(name)
		if !ok {
			continue
		}

		tag := f.Tag
		if tag == "" {
			continue
		}

		var o, a string
		if a = tag.Get("arg"); a != "" {
			a = fmt.Sprintf("=%s", a)
		}
		if s := tag.Get("short"); s != "" {
			o = fmt.Sprintf("-%s, --%s%s", tag.Get("short"), tag.Get("long"), a)
		} else {
			o = fmt.Sprintf("--%s%s", tag.Get("long"), a)
		}

		desc := tag.Get("description")
		if i := strings.Index(desc, "\n"); i >= 0 {
			var buf bytes.Buffer
			buf.WriteString(desc[:i+1])
			desc = desc[i+1:]
			const indent = "                        "
			for {
				if i = strings.Index(desc, "\n"); i >= 0 {
					buf.WriteString(indent)
					buf.WriteString(desc[:i+1])
					desc = desc[i+1:]
					continue
				}
				break
			}
			if len(desc) > 0 {
				buf.WriteString(indent)
				buf.WriteString(desc)
			}
			desc = buf.String()
		}
		help = append(help, fmt.Sprintf("  %-40s %s", o, desc))
	}

	return help
}

// help shows help
func (c *CLI) showHelp() {
	opts := strings.Join(c.buildHelp([]string{
		"Config",
		"LogLevel",
	}), "\n")

	help := `
Usage: dewy [--version] [--help] command <options>

Commands:
  server   Keep the app server up to date
  assets   Keep assets up to date

Options:
%s
`
	fmt.Fprintf(c.outStream, help, opts)
}

func main() {
	cli := &CLI{outStream: os.Stdout, errStream: os.Stderr}
	cli.run(os.Args[1:])
}

// CLI executes for cli
func (c *CLI) run(a []string) {
	p := flags.NewParser(c, flags.PrintErrors|flags.PassDoubleDash)
	args, err := p.ParseArgs(a)
	if err != nil || c.Help {
		c.showHelp()
		os.Exit(ExitErr)
		return
	}

	if c.Version {
		fmt.Fprintf(c.errStream, "%s version %s\n", dewy.Name, dewy.Version)
		os.Exit(ExitOK)
		return
	}

	if len(args) == 0 {
		fmt.Fprintf(c.errStream, "command not specified\n")
		os.Exit(ExitErr)
		return
	}

	c.Command = args[0]

	if c.LogLevel != "" {
		c.LogLevel = strings.ToUpper(c.LogLevel)
	} else {
		c.LogLevel = "ERROR"
	}

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel(c.LogLevel),
		Writer:   c.errStream,
	}
	log.SetOutput(filter)

	job := func() {
		conf := dewy.DefaultConfig()
		conf.Repository = dewy.RepositoryConfig{
			Name:     "mox",
			Owner:    "linyows",
			Artifact: "darwin_amd64.zip",
		}
		conf.OverrideWithEnv()
		d := dewy.New(conf)
		if err := d.Run(); err != nil {
			fmt.Fprintf(c.errStream, "%s\n", err)
			os.Exit(ExitErr)
			return
		}
	}
	scheduler.Every(10).Seconds().NotImmediately().Run(job)
	runtime.Goexit()
}
