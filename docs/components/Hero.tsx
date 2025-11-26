import * as React from 'react';

export function Hero({ children }) {
  return (
    <>
      <div className="hero">
        {children}
      </div>
      <style jsx>
      {`
        .hero :global(.desc) {
          font-size: 1.5rem;
          padding-top: 1rem;
          padding-bottom: 1rem;
        }
        .hero :global(.heading) {
          font-size: 3rem;
          position: relative;
        }
        .hero :global(.heading):before {
          content: "";
          position: absolute;
          top: -1.5rem;
          left: -2rem;
          padding: 1rem;
          border-top: 1px solid var(--text-color);
          border-left: 1px solid var(--text-color);
          display: block;
          width: 50px;
          height: 50px;
          z-index: -1;
        }
        .hero :global(.heading):after {
          content: "";
          position: absolute;
          bottom: -1.5rem;
          right: -2rem;
          padding: 1rem;
          border-bottom: 1px solid var(--text-color);
          border-right: 1px solid var(--text-color);
          display: block;
          width: 50px;
          height: 50px;
          z-index: -1;
        }
        .hero :global(img) {
          display: block;
          margin: 0 auto;
          width: 80%;
          height: auto;
          position: relative;
        }
        .hero :global(.hero-image) {
          margin: 4rem 0 0;
          position: relative;
        }
        .hero :global(img):last-child {
          position: absolute;
          top: 40%;
          left: 0;
          width: 60%;
          margin-left: 20%;
          margin-right: 20%;
          height: auto;
          object-fit: cover;
        }
        @media screen and (max-width: 600px) {
          .hero :global(.heading) {
            font-size: 2rem;
          }
          .hero :global(.heading):before {
            top: -1rem;
            left: -1.5rem;
            padding: .5rem;
            width: 50px;
            height: 50px;
          }
          .hero :global(.heading):after {
            bottom: -1rem;
            right: -1.5rem;
            padding: .5rem;
            width: 50px;
            height: 50px;
          }
          .hero :global(.desc) {
            font-size: 1rem;
            padding-bottom: .5rem;
          }
          .hero :global(img) {
            width: 100%;
          }
        }
      `}
      </style>
    </>
  );
};