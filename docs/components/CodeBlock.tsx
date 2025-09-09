import Prism from 'prismjs';

import * as React from 'react';

export function CodeBlock({children, 'data-language': language}) {
  const ref = React.useRef(null);

  React.useEffect(() => {
    if (ref.current) Prism.highlightElement(ref.current, false);
  }, [children]);

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
            position: relative;
            -webkit-overflow-scrolling: touch;
            width: 100%;
            margin: 1rem 0;
            overflow-x: auto;
          }
          /* Override Prism styles */
          .code-block :global(pre[class*='language-']) {
            text-shadow: none;
            border-radius: 10px;
            font-size: .85rem;
            padding: 1.2rem 1.5rem;
            width: 100%;
          }
          .code {
            white-space: pre-wrap;
            word-wrap: break-word;
            max-width: 100%;
            min-width: 0;
            overflow-x: auto;
          }
        `}
      </style>
    </div>
  );
}