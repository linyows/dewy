import { useEffect, useRef } from 'react';
import mermaid from 'mermaid';

interface MermaidProps {
  children: string;
}

export function Mermaid({ children }: MermaidProps) {
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    mermaid.initialize({
      startOnLoad: true,
      theme: 'default',
      securityLevel: 'loose',
    });

    if (ref.current) {
      ref.current.innerHTML = children;
      mermaid.init(undefined, ref.current);
    }
  }, [children]);

  return <div ref={ref} className="mermaid" />;
}

export default Mermaid;