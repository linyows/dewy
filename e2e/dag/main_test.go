package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"
)

// fixture has the shape of e2e/test.yml in `probe dag --mermaid` output:
// setup jobs, start jobs (one pair started in order), the release, verify
// jobs, a variant that does not wait for the release, and follow-up jobs that
// depend on verify jobs.
const fixture = `flowchart LR
    subgraph check["Check credentials"]
        direction TB
        check_step0["GITHUB_TOKEN is not empty"]
        check_step1["SLACK_TOKEN is not empty"]
        check_step0 --> check_step1
    end
    subgraph build["Build dewy"]
        direction TB
        build_step0["Go build for {{ vars.command }}-{{ vars.variant }}"]
    end
    subgraph start_srv["Start server"]
        direction TB
        start_srv_step0["Start dewy"]
    end
    subgraph start_cache_a["Start server with shared cache (A)"]
        direction TB
        start_cache_a_step0["Start dewy"]
    end
    subgraph start_cache_b["Start server with shared cache (B)"]
        direction TB
        start_cache_b_step0["Start dewy"]
    end
    subgraph start_healthfail["Start container with a failing health check"]
        direction TB
        start_healthfail_step0["Start dewy"]
    end
    subgraph create_release["Create new version"]
        direction TB
        create_release_step0["Create release"]
        create_release_step1["Verify release exists"]
        create_release_step0 --> create_release_step1
    end
    subgraph job_7["Verify server"]
        direction TB
        job_7_step0["Server verify"]
    end
    subgraph verify_cache_a["Verify server with shared cache (A)"]
        direction TB
        verify_cache_a_step0["Server verify"]
    end
    subgraph verify_cache_b["Verify server with shared cache (B)"]
        direction TB
        verify_cache_b_step0["Server verify"]
    end
    subgraph job_10["Verify container health check failure handling"]
        direction TB
        job_10_step0["Container health check failure verify"]
    end
    subgraph job_11["Verify shared cache reduces upstream downloads"]
        direction TB
        job_11_step0["Count Cached artifact log lines"]
    end
    subgraph prepare_slot["Prepare slot versions"]
        direction TB
        prepare_slot_step0["Copy the new version to each slot"]
    end
    subgraph job_13["Remove slot versions"]
        direction TB
        job_13_step0["Remove"]
    end

    check --> build
    build --> start_srv
    build --> start_cache_a
    start_cache_a --> start_cache_b
    build --> start_healthfail
    start_srv --> create_release
    start_cache_b --> create_release
    create_release --> job_7
    create_release --> verify_cache_a
    create_release --> verify_cache_b
    start_healthfail --> job_10
    verify_cache_a --> job_11
    verify_cache_b --> job_11
    job_7 --> prepare_slot
    prepare_slot --> job_13
`

func TestPhase(t *testing.T) {
	tests := []struct {
		phase string
		want  string
	}{
		{
			phase: "start",
			want: `flowchart LR
    check["Check credentials"]
    build["Build dewy"]
    start_srv["Start server"]
    start_cache_a["Start server with shared cache (A)"]
    start_cache_b["Start server with shared cache (B)"]
    create_release["Create new version"]

    check --> build
    build --> start_srv
    build --> start_cache_a
    start_cache_a --> start_cache_b
    start_srv --> create_release
    start_cache_b --> create_release
`,
		},
		{
			// The health check failure variant depends on neither side of
			// the release, so it is shown with the verify jobs.
			phase: "verify",
			want: `flowchart LR
    start_healthfail["Start container with a failing health check"]
    create_release["Create new version"]
    job_7["Verify server"]
    verify_cache_a["Verify server with shared cache (A)"]
    verify_cache_b["Verify server with shared cache (B)"]
    job_10["Verify container health check failure handling"]

    create_release --> job_7
    create_release --> verify_cache_a
    create_release --> verify_cache_b
    start_healthfail --> job_10
`,
		},
		{
			// Follow-up jobs are shown with the verify jobs they depend on,
			// but not with the release.
			phase: "followup",
			want: `flowchart LR
    job_7["Verify server"]
    verify_cache_a["Verify server with shared cache (A)"]
    verify_cache_b["Verify server with shared cache (B)"]
    job_11["Verify shared cache reduces upstream downloads"]
    prepare_slot["Prepare slot versions"]
    job_13["Remove slot versions"]

    verify_cache_a --> job_11
    verify_cache_b --> job_11
    job_7 --> prepare_slot
    prepare_slot --> job_13
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.phase, func(t *testing.T) {
			g, err := parse(strings.NewReader(fixture))
			if err != nil {
				t.Fatal(err)
			}
			p, err := g.phase(tt.phase)
			if err != nil {
				t.Fatal(err)
			}
			var b bytes.Buffer
			if err := p.writeMermaid(&b); err != nil {
				t.Fatal(err)
			}
			if got := b.String(); got != tt.want {
				t.Errorf("phase %s:\n got:\n%s\nwant:\n%s", tt.phase, got, tt.want)
			}
		})
	}
}

// Every job appears in at least one phase, so no job is left out of the docs.
func TestPhasesCoverEveryJob(t *testing.T) {
	g, err := parse(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	shown := map[string]bool{}
	for _, name := range phases {
		p, err := g.phase(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range p.jobs {
			shown[id] = true
		}
	}
	for _, id := range g.jobs {
		if !shown[id] {
			t.Errorf("job %s is in no phase", id)
		}
	}
}

func TestParseErrors(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "flowchart LR\n", "no jobs found"},
		{"no release job", "flowchart LR\n    subgraph build[\"Build\"]\n    end\n", `job "create_release" not found`},
		{"unclosed subgraph", "flowchart LR\n    subgraph build[\"Build\"]\n        build_step0[\"Go build\"]\n", "subgraph without end"},
		{"malformed subgraph", "flowchart LR\n    subgraph build\n    end\n", "unexpected subgraph line"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parse(strings.NewReader(tt.input))
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("parse error = %v, want one containing %q", err, tt.want)
			}
		})
	}
}

func TestUnknownPhase(t *testing.T) {
	g, err := parse(strings.NewReader(fixture))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := g.phase("deploy"); err == nil {
		t.Error("phase(deploy) returned no error")
	}
}

// Usage errors are reported before probe is run, so they do not depend on
// probe being installed.
func TestRunUsage(t *testing.T) {
	for _, args := range [][]string{nil, {"deploy"}, {"start", "verify"}} {
		var stdout, stderr bytes.Buffer
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Errorf("run(%q) = %d, want 2", args, code)
		}
		if !strings.Contains(stderr.String(), "usage: dag") {
			t.Errorf("run(%q) stderr = %q, want usage", args, stderr.String())
		}
		if stdout.Len() != 0 {
			t.Errorf("run(%q) wrote %q to stdout, want nothing", args, stdout.String())
		}
	}
}

// Every job in e2e/test.yml appears in at least one phase. Skipped when probe
// is not installed.
func TestPhasesCoverEveryJobInTestYML(t *testing.T) {
	if _, err := exec.LookPath("probe"); err != nil {
		t.Skip("probe is not installed")
	}
	out, err := probeDag("../test.yml")
	if err != nil {
		t.Fatal(err)
	}
	g, err := parse(bytes.NewReader(out))
	if err != nil {
		t.Fatal(err)
	}
	shown := map[string]bool{}
	for _, name := range phases {
		p, err := g.phase(name)
		if err != nil {
			t.Fatal(err)
		}
		for _, id := range p.jobs {
			shown[id] = true
		}
	}
	for _, id := range g.jobs {
		if !shown[id] {
			t.Errorf("job %s (%s) is in no phase", id, g.labels[id])
		}
	}
}
