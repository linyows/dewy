package dewy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"reflect"
	"strings"

	"github.com/hashicorp/logutils"
	flags "github.com/jessevdk/go-flags"
	"github.com/linyows/dewy/repo"
)

const (
	// ExitOK for exit code
	ExitOK int = 0

	// ExitErr for exit code
	ExitErr int = 1
)

// CLI struct
type CLI struct {
	outStream, errStream io.Writer
	Command              string
	Args                 []string
	Config               string `long:"config" short:"c" description:"Path to configuration file"`
	LogLevel             string `long:"log-level" short:"l" arg:"(debug|info|warn|error)" description:"Level displayed as log"`
	Interval             int    `long:"interval" arg:"seconds" short:"i" description:"The polling interval to the repository (default: 10)"`
	Port                 string `long:"port" short:"p" description:"TCP port to listen"`
	Repository           string `long:"repository" short:"r" description:"Repository for application"`
	Artifact             string `long:"artifact" short:"a" description:"Artifact for application"`
	Help                 bool   `long:"help" short:"h" description:"show this help message and exit"`
	Version              bool   `long:"version" short:"v" description:"prints the version number"`
}

// RunCLI runs as CLI
func RunCLI(o, e io.Writer, a []string) int {
	cli := &CLI{outStream: o, errStream: e, Interval: -1}
	return cli.run(a)
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

func (c *CLI) showHelp() {
	opts := strings.Join(c.buildHelp([]string{
		"Config",
		"Interval",
		"Repository",
		"Artifact",
		"Port",
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

func (c *CLI) run(a []string) int {
	p := flags.NewParser(c, flags.PrintErrors|flags.PassDoubleDash)
	args, err := p.ParseArgs(a)
	if err != nil || c.Help {
		c.showHelp()
		return ExitErr
	}

	if c.Version {
		fmt.Fprintf(c.errStream, "%s version %s [%v, %v]\n", name, version, commit, date)
		return ExitOK
	}

	if len(args) == 0 || (args[0] != "server" && args[0] != "assets") {
		fmt.Fprintf(c.errStream, "Error: command is not available\n")
		c.showHelp()
		return ExitErr
	}

	if c.Interval < 0 {
		c.Interval = 10
	}

	c.Command = args[0]

	if len(args) > 1 {
		c.Args = args[1:]
	}

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

	conf := DefaultConfig()

	re := strings.Split(c.Repository, "/")
	conf.Repository = repo.Config{
		Name:     re[1],
		Owner:    re[0],
		Artifact: c.Artifact,
	}

	if c.Command == "server" {
		conf.Command = SERVER
		conf.Starter = &StarterConfig{
			ports:   []string{c.Port},
			command: c.Args[0],
			args:    c.Args[1:],
		}
	} else {
		conf.Command = ASSETS
	}

	conf.OverrideWithEnv()
	d := New(conf)

	d.Start(c.Interval)

	return ExitOK
}
