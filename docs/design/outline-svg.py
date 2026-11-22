"""Convert the <text> elements of an SVG to outlined <path> elements.

The source SVG keeps its text editable; the published SVG has no text, so it
renders the same on hosts without Avenir Next. Run on macOS, where the fonts
are installed:

    python3 docs/design/outline-svg.py docs/design/server-architecture.src.svg \
        docs/public/images/server-architecture.svg
"""

import re
import sys
import xml.etree.ElementTree as ET

from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen
from fontTools.ttLib import TTCollection

AVENIR = "/System/Library/Fonts/Avenir Next.ttc"
FALLBACK = "/System/Library/Fonts/ヒラギノ角ゴシック W5.ttc"

# SVG class -> index of the face in the Avenir Next collection.
FACES = {"t": 2, "s": 5}  # Demi Bold, Medium

SVG_NS = "http://www.w3.org/2000/svg"
ET.register_namespace("", SVG_NS)


class Face:
    def __init__(self, font):
        self.font = font
        self.cmap = font.getBestCmap()
        self.glyphs = font.getGlyphSet()
        self.upem = font["head"].unitsPerEm
        self.kern = {}
        if "kern" in font:
            for table in font["kern"].kernTables:
                self.kern.update(getattr(table, "kernTable", {}))


def load(path, index):
    return Face(TTCollection(path).fonts[index])


def layout(text, size, anchor, face, fallback):
    """Return the start offset and the (face, glyph, x) of each character."""
    runs = []
    pos = 0.0
    prev = None
    for ch in text:
        f = face if ord(ch) in face.cmap else fallback
        g = f.cmap[ord(ch)]
        scale = size / f.upem
        if prev is not None and prev[0] is f:
            pos += f.kern.get((prev[1], g), 0) * scale
        runs.append((f, g, pos))
        pos += f.glyphs[g].width * scale
        prev = (f, g)
    offset = {"middle": -pos / 2, "end": -pos}.get(anchor, 0.0)
    return offset, runs


def num(v):
    return f"{v:.2f}".rstrip("0").rstrip(".")


def main(src, dst):
    faces = {cls: load(AVENIR, i) for cls, i in FACES.items()}
    fallback = load(FALLBACK, 0)
    ids = {}
    defs = ET.Element(f"{{{SVG_NS}}}defs")

    def glyph_id(f, g):
        key = (id(f), g)
        if key not in ids:
            ids[key] = f"g{len(ids)}"
            pen = SVGPathPen(f.glyphs)
            f.glyphs[g].draw(pen)
            ET.SubElement(defs, f"{{{SVG_NS}}}path", {"id": ids[key], "d": pen.getCommands()})
        return ids[key]

    tree = ET.parse(src)
    root = tree.getroot()
    for parent in tree.iter():
        for i, el in enumerate(list(parent)):
            if el.tag != f"{{{SVG_NS}}}text":
                continue
            cls = el.get("class")
            size = float(el.get("font-size"))
            x, y = float(el.get("x")), float(el.get("y"))
            offset, runs = layout("".join(el.itertext()), size,
                                  el.get("text-anchor", "start"), faces[cls], fallback)
            group = ET.Element(f"{{{SVG_NS}}}g", {"class": cls})
            if el.get("style"):
                group.set("style", el.get("style"))
            for f, g, pos in runs:
                if not f.glyphs[g].width or g == f.cmap.get(ord(" ")):
                    continue
                scale = size / f.upem
                ET.SubElement(group, f"{{{SVG_NS}}}use", {
                    "href": "#" + glyph_id(f, g),
                    "transform": f"matrix({scale:.6g} 0 0 {-scale:.6g} {num(x + offset + pos)} {num(y)})",
                })
            group.tail = el.tail
            parent.remove(el)
            parent.insert(i, group)
    root.insert(1, defs)
    out = ET.tostring(root, encoding="unicode")
    out = re.sub(r"\.([ts]) \{ font-family:[^;]*; font-weight: \d+; ", r".\1 { ", out)
    out = "\n".join(line.rstrip() for line in out.splitlines())
    with open(dst, "w") as fh:
        fh.write('<?xml version="1.0" encoding="UTF-8" standalone="no"?>\n' + out + "\n")


if __name__ == "__main__":
    main(sys.argv[1], sys.argv[2])
