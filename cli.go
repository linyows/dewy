package dewy

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"reflect"
	"sort"
	"strconv"
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
	LogLevel         string   `long:"log-level" short:"l" arg:"(debug|info|warn|error)" description:"Set log level for output"`
	Interval         int      `long:"interval" arg:"seconds" short:"i" description:"Polling interval in seconds for checking registry updates (default: 10)"`
	Ports            []string `long:"port" short:"p" description:"TCP ports for server command to listen on (comma-separated, ranges, or multiple flags)"`
	Registry         string   `long:"registry" description:"Registry URL (e.g., ghr://owner/repo, s3://region/bucket/prefix)"`
	Notify           string   `long:"notify" description:"[DEPRECATED] Use --notifier instead"`
	Notifier         string   `long:"notifier" description:"Notifier URL for deployment notifications (e.g., slack://channel, mail://smtp:port/recipient)"`
	BeforeDeployHook string   `long:"before-deploy-hook" description:"Shell command to execute before deployment begins"`
	AfterDeployHook  string   `long:"after-deploy-hook" description:"Shell command to execute after successful deployment"`
	Help             bool     `long:"help" short:"h" description:"show this help message and exit"`
	Version          bool     `long:"version" short:"v" description:"prints the version number"`
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
		"Notifier",
		"Ports",
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

	PrintVersion(c.env.Out, c.env.Version, c.env.Commit, c.env.Date)
	conf := DefaultConfig()

	if c.Registry == "" {
		fmt.Fprintf(c.env.Err, "Error: --registry is not set\n")
		c.showHelp()
		return ExitErr
	}
	conf.Registry = c.Registry
	// Handle notifier argument with backward compatibility
	if c.Notifier != "" {
		conf.Notifier = c.Notifier
	} else if c.Notify != "" {
		fmt.Fprintf(c.env.Err, "⚠️ notify argument is deprecated and will be removed. Use notifier instead.\n")
		conf.Notifier = c.Notify
	}
	conf.BeforeDeployHook = c.BeforeDeployHook
	conf.AfterDeployHook = c.AfterDeployHook

	if c.command == "server" {
		conf.Command = SERVER
		parsedPorts, err := parsePorts(c.Ports)
		if err != nil {
			fmt.Fprintf(c.env.Err, "Error: invalid port specification: %s\n", err)
			return ExitErr
		}
		conf.Starter = &StarterConfig{
			ports:   parsedPorts,
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

// parsePorts parses port specifications from CLI arguments
func parsePorts(portSpecs []string) ([]string, error) {
	if len(portSpecs) == 0 {
		return nil, nil
	}

	var allPorts []string
	for _, spec := range portSpecs {
		ports, err := parsePortSpec(spec)
		if err != nil {
			return nil, err
		}
		allPorts = append(allPorts, ports...)
	}

	// Remove duplicates and validate
	return validateAndDeduplicatePorts(allPorts)
}

// parsePortSpec parses a single port specification (supports comma-separated and ranges)
func parsePortSpec(spec string) ([]string, error) {
	if spec == "" {
		return nil, nil
	}

	var ports []string
	parts := strings.Split(spec, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		if strings.Contains(part, "-") {
			// Range specification (e.g., "8000-8005")
			rangePorts, err := parsePortRange(part)
			if err != nil {
				return nil, err
			}
			ports = append(ports, rangePorts...)
		} else {
			// Single port
			if err := validatePort(part); err != nil {
				return nil, err
			}
			ports = append(ports, part)
		}
	}

	return ports, nil
}

// parsePortRange parses a port range like "8000-8005"
func parsePortRange(rangeSpec string) ([]string, error) {
	parts := strings.Split(rangeSpec, "-")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid port range format: %s", rangeSpec)
	}

	startStr := strings.TrimSpace(parts[0])
	endStr := strings.TrimSpace(parts[1])

	start, err := strconv.Atoi(startStr)
	if err != nil {
		return nil, fmt.Errorf("invalid start port in range %s: %v", rangeSpec, err)
	}

	end, err := strconv.Atoi(endStr)
	if err != nil {
		return nil, fmt.Errorf("invalid end port in range %s: %v", rangeSpec, err)
	}

	if start > end {
		return nil, fmt.Errorf("start port (%d) cannot be greater than end port (%d)", start, end)
	}

	if end-start > 100 {
		return nil, fmt.Errorf("port range too large (%d ports), maximum allowed is 100", end-start+1)
	}

	var ports []string
	for i := start; i <= end; i++ {
		if err := validatePortNumber(i); err != nil {
			return nil, fmt.Errorf("invalid port %d in range %s: %v", i, rangeSpec, err)
		}
		ports = append(ports, strconv.Itoa(i))
	}

	return ports, nil
}

// validatePort validates a port string
func validatePort(port string) error {
	portNum, err := strconv.Atoi(port)
	if err != nil {
		return fmt.Errorf("invalid port number: %s", port)
	}
	return validatePortNumber(portNum)
}

// validatePortNumber validates a port number
func validatePortNumber(port int) error {
	if port < 1 || port > 65535 {
		return fmt.Errorf("port number must be between 1 and 65535, got %d", port)
	}
	if port < 1024 {
		log.Printf("[WARN] Using privileged port %d (< 1024) may require root privileges", port)
	}
	return nil
}

// validateAndDeduplicatePorts removes duplicates and sorts ports
func validateAndDeduplicatePorts(ports []string) ([]string, error) {
	if len(ports) == 0 {
		return ports, nil
	}

	// Use map to track unique ports
	uniquePorts := make(map[string]bool)
	for _, port := range ports {
		uniquePorts[port] = true
	}

	// Convert back to slice and sort
	var result []string
	for port := range uniquePorts {
		result = append(result, port)
	}

	// Sort numerically
	sort.Slice(result, func(i, j int) bool {
		a, _ := strconv.Atoi(result[i])
		b, _ := strconv.Atoi(result[j])
		return a < b
	})

	return result, nil
}
