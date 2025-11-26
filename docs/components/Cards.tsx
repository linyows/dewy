import * as React from 'react';

export function Cards({ children }) {
  return (
    <>
      <div className="cards">
        {children}
      </div>
      <style jsx>
      {`
        .cards {
          margin: 2rem 0;
        }
        .cards :global(ul) {
          display: grid;
          grid-template-columns: repeat(4, 1fr);
          gap: 0;
          list-style: none;
          margin: 0;
          padding: 0;
          border: 1px solid var(--text-color);
          border-left: none;
          border-right: none;
        }
        .cards :global(li) {
          padding: 4rem 2rem 3rem;
          padding-left: 10%;
          min-padding-left: 2rem;
          padding-right: 10%;
          min-padding-right: 2rem;
          background-color: var(--secondary-color);
        }
        .cards :global(.heading) {
          margin: 0;
          padding: 0;
          font-size: 1.8rem;
        }
        .cards :global(img) {
          display: block;
          width: 60%;
          max-width: 300px;
          height: auto;
          margin: 2rem auto;
        }
        @media screen and (max-width: 1300px) {
          .cards :global(ul) {
            grid-template-columns: repeat(2, 1fr);
          }
        }
        @media screen and (max-width: 600px) {
          .cards :global(ul) {
            grid-template-columns: repeat(1, 1fr);
          }
        }
      }
      `}
      </style>
    </>
  );
};