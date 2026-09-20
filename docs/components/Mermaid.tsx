import { useEffect, useRef } from 'react';
import mermaid from 'mermaid';

interface MermaidProps {
  children: string;
}

// Falls back to the literal values in public/globals.css when the custom
// property cannot be read, so the diagram never renders with mermaid's own
// purple defaults.
function paletteColor(name: string, fallback: string) {
  const value = getComputedStyle(document.documentElement)
    .getPropertyValue(name)
    .trim();
  return value || fallback;
}

export function Mermaid({ children }: MermaidProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const black = paletteColor('--text-color', 'rgba(60, 66, 87, 1)');
    const pink = paletteColor('--secondary-color', 'rgb(250, 218, 221)');
    const yellow = paletteColor('--accent-color', 'rgb(250, 218, 94)');
    const white = '#ffffff';

    mermaid.initialize({
      startOnLoad: false,
      theme: 'base',
      securityLevel: 'loose',
      // Render flowcharts at their natural size and let the container scroll.
      // Shrinking a wide DAG to the article width makes its labels unreadable.
      flowchart: { useMaxWidth: false },
      themeVariables: {
        // Strokes: lifelines, arrows, node and box borders.
        lineColor: black,
        arrowheadColor: black,
        signalColor: black,
        actorLineColor: black,
        actorBorder: black,
        primaryBorderColor: black,
        labelBoxBorderColor: black,
        activationBorderColor: black,
        noteBorderColor: black,

        // Fills: actors, nodes, alt/loop label boxes, activations.
        primaryColor: pink,
        secondaryColor: pink,
        tertiaryColor: white,
        actorBkg: pink,
        labelBoxBkgColor: pink,
        activationBkgColor: pink,

        // Notes.
        noteBkgColor: yellow,
        noteTextColor: black,

        // Text.
        textColor: black,
        primaryTextColor: black,
        actorTextColor: black,
        signalTextColor: black,
        labelTextColor: black,
        loopTextColor: black,
      },
    });

    if (ref.current) {
      ref.current.removeAttribute('data-processed');
      ref.current.innerHTML = children;
      mermaid.run({ nodes: [ref.current] });
    }
  }, [children]);

  return <div ref={ref} className="mermaid" />;
}

export default Mermaid;
