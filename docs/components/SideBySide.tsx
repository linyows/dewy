import * as React from 'react';

export function SideBySide({ children }) {
  const [first, ...rest] = React.Children.toArray(children);
  return (
    <div className="side-by-side">
      <div className="left column equal-width">{first}</div>
      <div className="right column equal-width">{rest}</div>
      <style jsx>
        {`
          .side-by-side {
            width: 100%;
            padding: 0;
            margin: 1rem 0;
            display: flex;
          }
          .column {
            overflow: auto;
          }
          .left {
            padding-right: 3rem;
          }
          .right {
            padding-left: 3rem;
          }
          .side-by-side :global(.heading) {
            font-size: 3rem;
            margin: 0;
          }
          .side-by-side :global(img) {
            width: 100%;
            height: auto;
            max-width: 763px;
            display: block;
            margin: 4rem auto 0;
          }
          @media screen and (max-width: 1000px) {
            .side-by-side {
              flex-direction: column;
            }
            .side-by-side :global(.heading) {
              font-size: 2rem;
            }
            .column {
              overflow: initial;
            }
            .left {
              padding: 0;
              border: none;
            }
            .right {
              padding-top: 1rem;
              padding-left: 0rem;
            }
            .side-by-side :global(img) {
              max-width: 500px;
            }
          }
          @media screen and (max-width: 600px) {
            .side-by-side :global(img) {
              margin-top: 1.5rem;
              width: 60%;
            }
          }
        `}
      </style>
    </div>
  );
};