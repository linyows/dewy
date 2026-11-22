import * as React from 'react';

export function Caption({ children }) {
  return (
    <>
      <div className="caption">
        {children}
      </div>
      <style jsx>
      {`
        .caption {
          font-size: .85rem;
          line-height: 1.6;
          opacity: .75;
          margin-top: -.5rem;
          margin-bottom: 1.5rem;
        }
        .caption :global(p),
        .caption :global(ol),
        .caption :global(ul) {
          margin: .25rem 0;
        }
      `}
      </style>
    </>
  );
}
