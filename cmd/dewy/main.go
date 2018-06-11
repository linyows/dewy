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
	"time"

	"github.com/carlescere/scheduler"
	"github.com/hashicorp/logutils"
	flags "github.com/jessevdk/go-flags"
	starter "github.com/lestrrat-go/server-starter"
	"github.com/linyows/dewy"
)

const (
	// ExitOK for exit code
	ExitOK int = 0

	// ExitErr for exit code
	ExitErr int = 1
)

type StarterConfig struct {
	args       []string
	command    string
	dir        string
	interval   int
	pidfile    string
	ports      []string
	paths      []string
	sigonhup   string
	sigonterm  string
	statusfile string
}

func (c StarterConfig) Args() []string          { return c.args }
func (c StarterConfig) Command() string         { return c.command }
func (c StarterConfig) Dir() string             { return c.dir }
func (c StarterConfig) Interval() time.Duration { return time.Duration(c.interval) * time.Second }
func (c StarterConfig) PidFile() string         { return c.pidfile }
func (c StarterConfig) Ports() []string         { return c.ports }
func (c StarterConfig) Paths() []string         { return c.paths }
func (c StarterConfig) SignalOnHUP() os.Signal  { return starter.SigFromName(c.sigonhup) }
func (c StarterConfig) SignalOnTERM() os.Signal { return starter.SigFromName(c.sigonterm) }
func (c StarterConfig) StatusFile() string      { return c.statusfile }

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

func main() {
	cli := &CLI{outStream: os.Stdout, errStream: os.Stderr, Interval: -1}
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

	job := func() {
		conf := dewy.DefaultConfig()

		repo := strings.Split(c.Repository, "/")
		conf.Repository = dewy.RepositoryConfig{
			Name:     repo[1],
			Owner:    repo[0],
			Artifact: c.Artifact,
		}
		conf.Starter = &StarterConfig{
			ports:   []string{c.Port},
			command: c.Command,
			args:    c.Args,
		}
		conf.OverrideWithEnv()
		d := dewy.New(conf)
		if err := d.Run(); err != nil {
			fmt.Fprintf(c.errStream, "%s\n", err)
			os.Exit(ExitErr)
			return
		}
	}

	scheduler.Every(c.Interval).Seconds().NotImmediately().Run(job)
	runtime.Goexit()
}
