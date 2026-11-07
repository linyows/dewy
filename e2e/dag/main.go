// Command dag prints one phase of the e2e job graph as a Mermaid flowchart,
// for the docs.
//
// The full graph from `probe dag --mermaid e2e/test.yml` includes every step
// and is too large to read on a page. dag collapses each job into one node and
// splits the jobs by their dependencies on the create_release job:
//
//	start     jobs create_release depends on, and create_release itself
//	          (Phase 0 to 2)
//	verify    create_release and the jobs that depend on it directly, plus
//	          the jobs that depend on neither (the health check failure
//	          variant) (Phase 3)
//	followup  jobs that depend on create_release through a verify job, with
//	          those verify jobs (Phase 4 to 6)
//
// The phases are found from the dependencies rather than from job names or
// IDs, because a job without an id gets one such as job_19 that shifts when
// jobs are added.
//
// Usage, from the repository root:
//
//	go run ./e2e/dag [-workflow e2e/test.yml] start|verify|followup
package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
)

// rootJob is the job the phases are split around. It has an explicit id in
// e2e/test.yml, so its id does not change when jobs are added.
const rootJob = "create_release"

var phases = []string{"start", "verify", "followup"}

type edge struct {
	from, to string
}

// graph is the job-level dependency graph, with jobs in the order Probe
// printed them.
type graph struct {
	jobs   []string
	labels map[string]string
	edges  []edge
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("dag", flag.ContinueOnError)
	fs.SetOutput(stderr)
	workflow := fs.String("workflow", "e2e/test.yml", "Probe workflow file")
	fs.Usage = func() {
		fmt.Fprintf(stderr, "usage: dag [-workflow file] %s\n", strings.Join(phases, "|"))
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 || !slices.Contains(phases, fs.Arg(0)) {
		fs.Usage()
		return 2
	}

	out, err := probeDag(*workflow)
	if err != nil {
		fmt.Fprintf(stderr, "dag: %v\n", err)
		return 1
	}
	g, err := parse(bytes.NewReader(out))
	if err != nil {
		fmt.Fprintf(stderr, "dag: %v\n", err)
		return 1
	}
	p, err := g.phase(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "dag: %v\n", err)
		return 1
	}
	if err := p.writeMermaid(stdout); err != nil {
		fmt.Fprintf(stderr, "dag: %v\n", err)
		return 1
	}
	return 0
}

// probeDag returns the output of `probe dag --mermaid` for the workflow.
func probeDag(workflow string) ([]byte, error) {
	var stderr bytes.Buffer
	cmd := exec.Command("probe", "dag", "--mermaid", workflow) //nolint:gosec // G204: the workflow path comes from the developer running this tool
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("probe dag --mermaid %s: %w: %s", workflow, err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// parse reads the output of `probe dag --mermaid`. Each job is a subgraph
// whose steps are dropped, and each dependency between jobs is an edge
// outside the subgraphs:
//
//	flowchart LR
//	    subgraph build["Build dewy"]
//	        direction TB
//	        build_step0["Go build"]
//	    end
//
//	    check --> build
func parse(r io.Reader) (*graph, error) {
	g := &graph{labels: map[string]string{}}
	inJob := false

	sc := bufio.NewScanner(r)
	for sc.Scan() {
		line := sc.Text()
		trimmed := strings.TrimSpace(line)

		if inJob {
			if trimmed == "end" {
				inJob = false
			}
			continue
		}

		if rest, ok := strings.CutPrefix(trimmed, "subgraph "); ok {
			id, label, ok := strings.Cut(rest, "[")
			if !ok || !strings.HasSuffix(label, "]") {
				return nil, fmt.Errorf("unexpected subgraph line: %q", line)
			}
			g.jobs = append(g.jobs, id)
			g.labels[id] = strings.TrimSuffix(label, "]")
			inJob = true
			continue
		}

		if from, to, ok := strings.Cut(trimmed, " --> "); ok {
			g.edges = append(g.edges, edge{from: from, to: to})
		}
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	if inJob {
		return nil, errors.New("subgraph without end")
	}
	if len(g.jobs) == 0 {
		return nil, errors.New("no jobs found")
	}
	if _, ok := g.labels[rootJob]; !ok {
		return nil, fmt.Errorf("job %q not found", rootJob)
	}
	return g, nil
}

// phase returns the subgraph of the jobs in the named phase. See the package
// comment for what each phase holds.
func (g *graph) phase(name string) (*graph, error) {
	ancestors := g.reach(rootJob, func(e edge) (string, string) { return e.to, e.from })
	descendants := g.reach(rootJob, func(e edge) (string, string) { return e.from, e.to })
	children := map[string]bool{}
	for _, e := range g.edges {
		if e.from == rootJob {
			children[e.to] = true
		}
	}

	keep := map[string]bool{}
	for _, id := range g.jobs {
		switch name {
		case "start":
			keep[id] = ancestors[id]
		case "verify":
			keep[id] = id == rootJob || children[id] || (!ancestors[id] && !descendants[id])
		case "followup":
			keep[id] = descendants[id] && id != rootJob && !children[id]
		default:
			return nil, fmt.Errorf("unknown phase %q", name)
		}
	}
	// followup also shows the verify jobs it depends on.
	if name == "followup" {
		var verifies []string
		for _, e := range g.edges {
			if children[e.from] && keep[e.to] {
				verifies = append(verifies, e.from)
			}
		}
		for _, id := range verifies {
			keep[id] = true
		}
	}

	return g.subgraph(func(id string) bool { return keep[id] }), nil
}

// reach returns the jobs reachable from start, start included, following the
// edges in the direction step gives: step returns the end an edge is
// followed from and the end it leads to.
func (g *graph) reach(start string, step func(edge) (string, string)) map[string]bool {
	seen := map[string]bool{start: true}
	queue := []string{start}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, e := range g.edges {
			from, to := step(e)
			if from == cur && !seen[to] {
				seen[to] = true
				queue = append(queue, to)
			}
		}
	}
	return seen
}

// subgraph returns the jobs keep accepts and the edges between them, in
// their original order.
func (g *graph) subgraph(keep func(string) bool) *graph {
	s := &graph{labels: map[string]string{}}
	for _, id := range g.jobs {
		if keep(id) {
			s.jobs = append(s.jobs, id)
			s.labels[id] = g.labels[id]
		}
	}
	for _, e := range g.edges {
		if keep(e.from) && keep(e.to) {
			s.edges = append(s.edges, e)
		}
	}
	return s
}

// writeMermaid writes the graph as a left-to-right Mermaid flowchart. Labels
// are written as Probe printed them, quotes included.
func (g *graph) writeMermaid(w io.Writer) error {
	var b strings.Builder
	b.WriteString("flowchart LR\n")
	for _, id := range g.jobs {
		fmt.Fprintf(&b, "    %s[%s]\n", id, g.labels[id])
	}
	b.WriteString("\n")
	for _, e := range g.edges {
		fmt.Fprintf(&b, "    %s --> %s\n", e.from, e.to)
	}
	_, err := io.WriteString(w, b.String())
	return err
}
