import Prism from 'prismjs';
import * as React from 'react';
import dynamic from 'next/dynamic';

const Mermaid = dynamic(() => import('./Mermaid'), { ssr: false });

export function CodeBlock({children, 'data-language': language}) {
  const ref = React.useRef(null);

  React.useEffect(() => {
    if (ref.current) Prism.highlightElement(ref.current, false);
  }, [children]);

  if (language === 'mermaid') {
    return <Mermaid>{children}</Mermaid>;
  }

  return (
    <div className="code-block" aria-live="polite">
      <pre className={`language-${language}`}>
        <code ref={ref} className="code">
          {children}
        </code>
      </pre>
      <style jsx>
        {`
          .code-block {
            width: 100%;
            position: relative;
            top: 0;
            left: 0;
            -webkit-overflow-scrolling: touch;
            max-width: 100%;
            min-width: var(--min-width);
            margin: 1rem 0;
          }
          /* Override Prism styles */
          .code-block :global(pre[class*='language-']) {
            text-shadow: none;
            border-radius: 4px;
            font-size: .85rem;
            padding: 1.2rem 1.5rem;
            width: 100%;
            overflow: auto;
            background-color: rgba(170, 170, 170, 0.1);
            border: 1px solid var(--text-dim-color);
            box-shadow: 5px 5px 1px rgba(0, 0, 0, 0.1);
          }
          .code-block code {
            white-space: pre;
            word-wrap: break-word;
            width: 100%;
          }
        `}
      </style>
    </div>
  );
}