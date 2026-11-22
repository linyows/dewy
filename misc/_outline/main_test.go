package main

import (
	"bytes"
	"encoding/xml"
	"os"
	"strings"
	"testing"

	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

func TestNum(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "0"},
		{-0.001, "0"},
		{1.5, "1.5"},
		{1.239, "1.24"},
		{100, "100"},
		{-2.10, "-2.1"},
	}
	for _, tt := range tests {
		if got := num(tt.in); got != tt.want {
			t.Errorf("num(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestStyleApply(t *testing.T) {
	base := style{face: "t", size: 16}
	s, dy := base.apply([]xml.Attr{
		{Name: xml.Name{Local: "class"}, Value: "b list"},
		{Name: xml.Name{Local: "dy"}, Value: "-9"},
	})
	if s.face != "b" || s.size != 29.167 || s.ls != -0.7 || dy != -9 {
		t.Errorf("class: got %+v dy=%v", s, dy)
	}
	s, _ = s.apply([]xml.Attr{{Name: xml.Name{Local: "style"}, Value: "font-size:14px;"}})
	if s.face != "b" || s.size != 14 || s.ls != -0.7 {
		t.Errorf("inline font-size: got %+v", s)
	}
}

func TestParseTexts(t *testing.T) {
	src := `<svg><g class="list"><text class="t" x="10" y="20">A<tspan class="b" dy="-5">B</tspan>C</text></g>` +
		`<text class="t step" x="1" y="2" text-anchor="middle">D</text></svg>`
	texts, err := parseTexts([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	if len(texts) != 2 {
		t.Fatalf("got %d texts, want 2", len(texts))
	}

	first := texts[0]
	if first.x != 10 || first.y != 20 || first.anchor != "" {
		t.Errorf("position: got %+v", first)
	}
	want := []run{
		{text: "A", style: style{face: "t", size: 29.167, ls: -0.7}},
		{text: "B", style: style{face: "b", size: 29.167, ls: -0.7}, dy: -5},
		{text: "C", style: style{face: "t", size: 29.167, ls: -0.7}},
	}
	if len(first.runs) != len(want) {
		t.Fatalf("got %d runs, want %d", len(first.runs), len(want))
	}
	for i, r := range first.runs {
		if r != want[i] {
			t.Errorf("run %d: got %+v, want %+v", i, r, want[i])
		}
	}
	if got := src[first.start:first.end]; !strings.HasPrefix(got, "<text") || !strings.HasSuffix(got, "</text>") {
		t.Errorf("offsets cover %q", got)
	}

	second := texts[1]
	if second.anchor != "middle" || second.runs[0].style.size != 20.833 {
		t.Errorf("second: got %+v", second)
	}
}

func TestPathData(t *testing.T) {
	p := func(x, y int) fixed.Point26_6 { return fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y)} }
	segs := sfnt.Segments{
		{Op: sfnt.SegmentOpMoveTo, Args: [3]fixed.Point26_6{p(0, 0)}},
		{Op: sfnt.SegmentOpLineTo, Args: [3]fixed.Point26_6{p(10, 0)}},
		{Op: sfnt.SegmentOpQuadTo, Args: [3]fixed.Point26_6{p(10, 10), p(0, 10)}},
		{Op: sfnt.SegmentOpMoveTo, Args: [3]fixed.Point26_6{p(2, 2)}},
		{Op: sfnt.SegmentOpCubeTo, Args: [3]fixed.Point26_6{p(3, 3), p(4, 4), p(5, 5)}},
	}
	want := "M0 0L10 0Q10 10 0 10ZM2 2C3 3 4 4 5 5Z"
	if got := pathData(segs); got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got := pathData(nil); got != "" {
		t.Errorf("empty: got %q", got)
	}
}

// TestOutline converts the real source SVG. It needs the macOS system fonts
// and is skipped elsewhere.
func TestOutline(t *testing.T) {
	for _, spec := range faces {
		if _, err := os.Stat(spec.path); err != nil {
			t.Skipf("font not available: %s", spec.path)
		}
	}
	src, err := os.ReadFile("../dewy-architecture.src.svg")
	if err != nil {
		t.Fatal(err)
	}
	o, err := newOutliner()
	if err != nil {
		t.Fatal(err)
	}
	out, err := o.outline(src)
	if err != nil {
		t.Fatal(err)
	}

	for _, s := range []string{"<text", "<tspan", "font-family"} {
		if bytes.Contains(out, []byte(s)) {
			t.Errorf("output still contains %q", s)
		}
	}
	for _, s := range []string{"<defs>", "<use xlink:href=", `xmlns:xlink="http://www.w3.org/1999/xlink"`, ".ln {"} {
		if !bytes.Contains(out, []byte(s)) {
			t.Errorf("output lacks %q", s)
		}
	}
	if err := xml.Unmarshal(out, new(struct{})); err != nil {
		t.Errorf("output is not well-formed XML: %v", err)
	}
	// Every glyph a <use> references has a definition.
	for _, id := range o.ids {
		for _, v := range id {
			if v != "" && !bytes.Contains(out, []byte(`<path id="`+v+`"`)) {
				t.Errorf("missing definition for %s", v)
			}
		}
	}
}
