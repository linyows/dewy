// Command main outlines the text of the architecture diagram.
//
// misc/dewy-architecture.src.svg keeps its labels as <text> elements so they
// stay editable. This command converts each of them into glyph outlines, so
// the published SVG renders the same without Avenir Next or Hiragino Sans
// installed. Run it from the repository root on macOS, where those fonts are:
//
//	go -C misc/_outline run .
//
// This directory is a module of its own, and its name starts with "_" so that
// "./..." in the dewy module skips it: the tool and its golang.org/x/image
// dependency stay out of dewy's build, tests and go.mod. Paths are relative to
// this directory.
//
// Each glyph is defined once under <defs> in 1/1000 em units and placed with
// <use>, which keeps the output small. Kerning comes from the font.
package main

import (
	"bytes"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
)

const (
	fill = "rgb(35,31,32)"
	// glyphUnits is the em size the glyph outlines under <defs> are written in.
	glyphUnits = 1000
)

// faceSpec names a font face by its file and PostScript name.
type faceSpec struct {
	path       string
	postScript string
}

// faces maps the CSS classes of the source SVG to font faces.
var faces = map[string]faceSpec{
	"t":   {"/System/Library/Fonts/Avenir Next.ttc", "AvenirNext-Medium"},
	"b":   {"/System/Library/Fonts/Avenir Next.ttc", "AvenirNext-Bold"},
	"d":   {"/System/Library/Fonts/Avenir Next.ttc", "AvenirNext-DemiBold"},
	"dot": {"/System/Library/Fonts/ヒラギノ角ゴシック W5.ttc", "HiraginoSans-W5"},
}

// sizes and spacings map the CSS classes of the source SVG to font-size and
// letter-spacing in px.
var (
	sizes    = map[string]float64{"list": 29.167, "box": 37.5, "step": 20.833, "note": 16}
	spacings = map[string]float64{"list": -0.7, "step": -0.3}
)

var fontSizeRe = regexp.MustCompile(`font-size:\s*([\d.]+)px`)

// style is the text style an element inherits from its ancestors.
type style struct {
	face string
	size float64
	ls   float64
}

// apply returns the style of an element with attrs, inheriting from s, and
// the element's dy.
func (s style) apply(attrs []xml.Attr) (style, float64) {
	var dy float64
	for _, a := range attrs {
		switch a.Name.Local {
		case "class":
			for c := range strings.FieldsSeq(a.Value) {
				if _, ok := faces[c]; ok {
					s.face = c
				}
				if v, ok := sizes[c]; ok {
					s.size = v
				}
				if v, ok := spacings[c]; ok {
					s.ls = v
				}
			}
		case "style":
			if m := fontSizeRe.FindStringSubmatch(a.Value); m != nil {
				if v, err := strconv.ParseFloat(m[1], 64); err == nil {
					s.size = v
				}
			}
		case "dy":
			if v, err := strconv.ParseFloat(a.Value, 64); err == nil {
				dy = v
			}
		}
	}
	return s, dy
}

// run is a stretch of characters drawn in one style. dy shifts the baseline
// before the run, and the shift stays for the runs after it.
type run struct {
	text  string
	style style
	dy    float64
}

// textElem is a <text> element of the source SVG.
type textElem struct {
	x, y   float64
	anchor string
	runs   []run
	// start and end are the byte offsets of the element in the source.
	start, end int64
}

// face is a loaded font face.
type face struct {
	font *sfnt.Font
	upm  float64
}

// outliner converts text into glyph outlines and collects the glyphs used.
type outliner struct {
	faces map[string]*face
	buf   sfnt.Buffer
	// defs maps a glyph id to its path data.
	defs map[string]string
	ids  map[string]map[sfnt.GlyphIndex]string
}

func newOutliner() (*outliner, error) {
	o := &outliner{
		faces: map[string]*face{},
		defs:  map[string]string{},
		ids:   map[string]map[sfnt.GlyphIndex]string{},
	}
	files := map[string]*sfnt.Collection{}
	for key, spec := range faces {
		c, ok := files[spec.path]
		if !ok {
			src, err := os.ReadFile(spec.path)
			if err != nil {
				return nil, fmt.Errorf("read font: %w", err)
			}
			c, err = sfnt.ParseCollection(src)
			if err != nil {
				return nil, fmt.Errorf("parse %s: %w", spec.path, err)
			}
			files[spec.path] = c
		}
		f, err := o.findFace(c, spec.postScript)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", spec.path, err)
		}
		o.faces[key] = &face{font: f, upm: float64(f.UnitsPerEm())}
		o.ids[key] = map[sfnt.GlyphIndex]string{}
	}
	return o, nil
}

func (o *outliner) findFace(c *sfnt.Collection, postScript string) (*sfnt.Font, error) {
	for i := 0; i < c.NumFonts(); i++ {
		f, err := c.Font(i)
		if err != nil {
			return nil, err
		}
		name, err := f.Name(&o.buf, sfnt.NameIDPostScript)
		if err == nil && name == postScript {
			return f, nil
		}
	}
	return nil, fmt.Errorf("face %s not found", postScript)
}

// placed is a glyph positioned on the page.
type placed struct {
	face  string
	index sfnt.GlyphIndex
	x, y  float64
	size  float64
}

// layout positions the glyphs of t and returns them with the advance width.
func (o *outliner) layout(t textElem) ([]placed, float64, error) {
	var glyphs []placed
	x, y := t.x, t.y
	for _, r := range t.runs {
		f, ok := o.faces[r.style.face]
		if !ok {
			return nil, 0, fmt.Errorf("unknown face %q", r.style.face)
		}
		ppem := fixed.Int26_6(f.upm * 64)
		scale := r.style.size / f.upm
		y += r.dy
		prev, hasPrev := sfnt.GlyphIndex(0), false
		for _, ch := range r.text {
			gi, err := f.font.GlyphIndex(&o.buf, ch)
			if err != nil {
				return nil, 0, err
			}
			if gi == 0 {
				return nil, 0, fmt.Errorf("no glyph for %q in %s", ch, r.style.face)
			}
			if hasPrev {
				k, err := f.font.Kern(&o.buf, prev, gi, ppem, font.HintingNone)
				if err != nil && !errors.Is(err, sfnt.ErrNotFound) {
					return nil, 0, err
				}
				x += float64(k) / 64 * scale
			}
			glyphs = append(glyphs, placed{face: r.style.face, index: gi, x: x, y: y, size: r.style.size})
			adv, err := f.font.GlyphAdvance(&o.buf, gi, ppem, font.HintingNone)
			if err != nil {
				return nil, 0, err
			}
			x += float64(adv)/64*scale + r.style.ls
			prev, hasPrev = gi, true
		}
	}
	return glyphs, x - t.x, nil
}

// glyphID registers the outline of a glyph once and returns its id. It
// returns "" for a glyph without an outline, such as a space.
func (o *outliner) glyphID(key string, gi sfnt.GlyphIndex) (string, error) {
	if id, ok := o.ids[key][gi]; ok {
		return id, nil
	}
	f := o.faces[key]
	segs, err := f.font.LoadGlyph(&o.buf, gi, fixed.Int26_6(glyphUnits*64), nil)
	if err != nil {
		return "", err
	}
	id := ""
	if d := pathData(segs); d != "" {
		name, err := f.font.GlyphName(&o.buf, gi)
		if err != nil || name == "" {
			name = fmt.Sprintf("g%d", gi)
		}
		id = key + "-" + sanitizeID(name)
		o.defs[id] = d
	}
	o.ids[key][gi] = id
	return id, nil
}

var idRe = regexp.MustCompile(`[^A-Za-z0-9_-]`)

func sanitizeID(s string) string { return idRe.ReplaceAllString(s, "_") }

// pathData converts glyph segments into SVG path data. The coordinates are
// in 1/glyphUnits em with the Y axis pointing down, as sfnt returns them.
func pathData(segs sfnt.Segments) string {
	var b strings.Builder
	pt := func(p fixed.Point26_6) string {
		return num(float64(p.X)/64) + " " + num(float64(p.Y)/64)
	}
	for i, s := range segs {
		switch s.Op {
		case sfnt.SegmentOpMoveTo:
			if i > 0 {
				b.WriteString("Z")
			}
			b.WriteString("M" + pt(s.Args[0]))
		case sfnt.SegmentOpLineTo:
			b.WriteString("L" + pt(s.Args[0]))
		case sfnt.SegmentOpQuadTo:
			b.WriteString("Q" + pt(s.Args[0]) + " " + pt(s.Args[1]))
		case sfnt.SegmentOpCubeTo:
			b.WriteString("C" + pt(s.Args[0]) + " " + pt(s.Args[1]) + " " + pt(s.Args[2]))
		}
	}
	if len(segs) > 0 {
		b.WriteString("Z")
	}
	return b.String()
}

// num formats v with at most two decimals and no trailing zeros.
func num(v float64) string {
	s := strconv.FormatFloat(v, 'f', 2, 64)
	s = strings.TrimRight(strings.TrimRight(s, "0"), ".")
	if s == "-0" || s == "" {
		return "0"
	}
	return s
}

// render returns the markup that replaces t: a group of <use> elements.
func (o *outliner) render(t textElem, indent string) (string, error) {
	glyphs, width, err := o.layout(t)
	if err != nil {
		return "", err
	}
	var shift float64
	switch t.anchor {
	case "middle":
		shift = -width / 2
	case "end":
		shift = -width
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<g style=\"fill:%s;\">", fill)
	for _, g := range glyphs {
		id, err := o.glyphID(g.face, g.index)
		if err != nil {
			return "", err
		}
		if id == "" {
			continue
		}
		s := strconv.FormatFloat(g.size/glyphUnits, 'g', 6, 64)
		fmt.Fprintf(&b, "\n%s    <use xlink:href=\"#%s\" transform=\"matrix(%s,0,0,%s,%s,%s)\"/>",
			indent, id, s, s, num(g.x+shift), num(g.y))
	}
	fmt.Fprintf(&b, "\n%s</g>", indent)
	return b.String(), nil
}

// parseTexts finds the <text> elements of src with the style each inherits.
func parseTexts(src []byte) ([]textElem, error) {
	dec := xml.NewDecoder(bytes.NewReader(src))
	stack := []style{{face: "t", size: 16}}
	var (
		texts []textElem
		cur   *textElem
		dy    float64
	)
	for {
		start := dec.InputOffset()
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		switch tk := tok.(type) {
		case xml.StartElement:
			s, d := stack[len(stack)-1].apply(tk.Attr)
			stack = append(stack, s)
			switch {
			case tk.Name.Local == "text":
				cur = &textElem{start: start}
				for _, a := range tk.Attr {
					switch a.Name.Local {
					case "x":
						cur.x, _ = strconv.ParseFloat(a.Value, 64)
					case "y":
						cur.y, _ = strconv.ParseFloat(a.Value, 64)
					case "text-anchor":
						cur.anchor = a.Value
					}
				}
			case cur != nil && tk.Name.Local == "tspan":
				dy += d
			}
		case xml.EndElement:
			stack = stack[:len(stack)-1]
			if tk.Name.Local == "text" && cur != nil {
				cur.end = dec.InputOffset()
				texts = append(texts, *cur)
				cur = nil
			}
		case xml.CharData:
			if cur != nil && len(tk) > 0 {
				cur.runs = append(cur.runs, run{text: string(tk), style: stack[len(stack)-1], dy: dy})
				dy = 0
			}
		}
	}
	return texts, nil
}

var styleRe = regexp.MustCompile(`(?s)(<style>)(.*?)(\n\s*</style>)`)

// outline returns src with every <text> element replaced by glyph outlines.
func (o *outliner) outline(src []byte) ([]byte, error) {
	texts, err := parseTexts(src)
	if err != nil {
		return nil, err
	}
	var out bytes.Buffer
	var pos int64
	for _, t := range texts {
		lineStart := bytes.LastIndexByte(src[:t.start], '\n') + 1
		indent := string(src[lineStart:t.start])
		g, err := o.render(t, indent)
		if err != nil {
			return nil, err
		}
		out.Write(src[pos:t.start])
		out.WriteString(g)
		pos = t.end
	}
	out.Write(src[pos:])
	res := out.Bytes()

	// Drop the font classes, which nothing uses any more, and put the
	// glyph outlines right after the style sheet.
	m := styleRe.FindSubmatchIndex(res)
	if m == nil {
		return nil, errors.New("no <style> element")
	}
	var kept []string
	for l := range strings.SplitSeq(string(res[m[4]:m[5]]), "\n") {
		if t := strings.TrimSpace(l); strings.HasPrefix(t, ".ln") || strings.HasPrefix(t, ".head") {
			kept = append(kept, l)
		}
	}
	ids := make([]string, 0, len(o.defs))
	for id := range o.defs {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var defs strings.Builder
	defs.WriteString("\n    <defs>")
	for _, id := range ids {
		fmt.Fprintf(&defs, "\n        <path id=\"%s\" d=\"%s\"/>", id, o.defs[id])
	}
	defs.WriteString("\n    </defs>")

	var b bytes.Buffer
	b.Write(res[:m[4]])
	b.WriteString("\n" + strings.Join(kept, "\n"))
	b.Write(res[m[6]:m[1]])
	b.WriteString(defs.String())
	b.Write(res[m[1]:])

	result := b.Bytes()
	if !bytes.Contains(result, []byte("xmlns:xlink=")) {
		result = bytes.Replace(result, []byte("<svg "),
			[]byte(`<svg xmlns:xlink="http://www.w3.org/1999/xlink" `), 1)
	}
	return result, nil
}

func main() {
	src := flag.String("src", "../dewy-architecture.src.svg", "source SVG with editable text")
	outs := flag.String("out", "../dewy-architecture.svg,../../docs/public/images/dewy-architecture.svg",
		"comma-separated paths to write the outlined SVG to")
	flag.Parse()

	in, err := os.ReadFile(*src)
	if err != nil {
		log.Fatal(err)
	}
	o, err := newOutliner()
	if err != nil {
		log.Fatal(err)
	}
	res, err := o.outline(in)
	if err != nil {
		log.Fatal(err)
	}
	for p := range strings.SplitSeq(*outs, ",") {
		if err := os.WriteFile(p, res, 0o644); err != nil {
			log.Fatal(err)
		}
		fmt.Printf("wrote %s (%d bytes)\n", p, len(res))
	}
}
