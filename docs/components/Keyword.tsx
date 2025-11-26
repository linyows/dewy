import * as React from 'react';

export function Keyword({ children }) {
  return (
    <>
      <div className="keyword">
        {children}
      </div>
      <style jsx>
      {`
        .keyword {
          padding-bottom: 2rem;
        }
        .keyword :global(p) {
          font-size: .85rem;
          padding: .3rem 1.5rem;
          padding-left: 2.5rem;
          border-radius: 50px;
          display: inline-block;
          margin: 0 auto;
          border: 1px solid var(--primary-color);
          color: var(--primary-color);
          position: relative;
        }
        .keyword :global(p):before {
          content: '⚫︎';
          display: block;
          position: absolute;
          top: .5rem;
          left: .8rem;
          font-size: 1.2rem;
          line-height: 1.2rem;
          color: var(--primary-color);
        }
      }
      `}
      </style>
    </>
  );
}