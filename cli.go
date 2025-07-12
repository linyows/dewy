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
)

const (
	// ExitOK for exit code.
	ExitOK int = 0

	// ExitErr for exit code.
	ExitErr int = 1
)

type cli struct {
	env              Env
	command          string
	args             []string
	LogLevel         string `long:"log-level" short:"l" arg:"(debug|info|warn|error)" description:"Level displayed as log"`
	Interval         int    `long:"interval" arg:"seconds" short:"i" description:"The polling interval to the repository (default: 10)"`
	Port             string `long:"port" short:"p" description:"TCP port to listen"`
	Registry         string `long:"registry" description:"Registry for application"`
	Notify           string `long:"notify" description:"Notify for application"`
	BeforeDeployHook string `long:"before-deploy-hook" description:"Command to execute before deploy"`
	AfterDeployHook  string `long:"after-deploy-hook" description:"Command to execute after deploy"`
	Help             bool   `long:"help" short:"h" description:"show this help message and exit"`
	Version          bool   `long:"version" short:"v" description:"prints the version number"`
}

// Env struct.
type Env struct {
	Out, Err io.Writer
	Args     []string
	Version  string
	Commit   string
	Date     string
}

// RunCLI runs as cli.
func RunCLI(env Env) int {
	cli := &cli{env: env, Interval: -1}
	return cli.run()
}

func (c *cli) buildHelp(names []string) []string {
	var help []string
	t := reflect.TypeOf(cli{})

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

func (c *cli) showHelp() {
	opts := strings.Join(c.buildHelp([]string{
		"Config",
		"Interval",
		"Registry",
		"Notify",
		"Port",
		"LogLevel",
		"BeforeDeployHook",
		"AfterDeployHook",
	}), "\n")

	help := `Usage: dewy [--version] [--help] command <options>

Commands:
  server   Keep the app server up to date
  assets   Keep assets up to date

Options:
%s
`
	Banner(c.env.Out)
	fmt.Fprintf(c.env.Out, help, opts)
}

func (c *cli) run() int {
	p := flags.NewParser(c, flags.PassDoubleDash)
	args, err := p.ParseArgs(c.env.Args)
	if err != nil || c.Help {
		c.showHelp()
		return ExitErr
	}

	if c.Version {
		fmt.Fprintf(c.env.Err, "dewy version %s [%v, %v]\n", c.env.Version, c.env.Commit, c.env.Date)
		return ExitOK
	}

	if len(args) == 0 || (args[0] != "server" && args[0] != "assets") {
		fmt.Fprintf(c.env.Err, "Error: command is not available\n")
		c.showHelp()
		return ExitErr
	}

	if c.Interval < 0 {
		c.Interval = 10
	}

	c.command = args[0]

	if len(args) > 1 {
		c.args = args[1:]
	}

	if c.LogLevel != "" {
		c.LogLevel = strings.ToUpper(c.LogLevel)
	} else {
		c.LogLevel = "ERROR"
	}

	filter := &logutils.LevelFilter{
		Levels:   []logutils.LogLevel{"DEBUG", "INFO", "WARN", "ERROR"},
		MinLevel: logutils.LogLevel(c.LogLevel),
		Writer:   c.env.Err,
	}
	log.SetOutput(filter)

	Banner(c.env.Out)
	conf := DefaultConfig()

	if c.Registry == "" {
		fmt.Fprintf(c.env.Err, "Error: --registry is not set\n")
		c.showHelp()
		return ExitErr
	}
	conf.Registry = c.Registry
	conf.Notify = c.Notify
	conf.BeforeDeployHook = c.BeforeDeployHook
	conf.AfterDeployHook = c.AfterDeployHook

	if c.command == "server" {
		conf.Command = SERVER
		conf.Starter = &StarterConfig{
			ports:   []string{c.Port},
			command: c.args[0],
			args:    c.args[1:],
		}
	} else {
		conf.Command = ASSETS
	}

	d, err := New(conf)
	if err != nil {
		fmt.Fprintf(c.env.Err, "Error: %s\n", err)
		return ExitErr
	}

	d.Start(c.Interval)

	return ExitOK
}
