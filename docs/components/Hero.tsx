import * as React from 'react';
import { VersionAnimation } from './VersionAnimation';

export function Hero({ children }) {
  const [first, ...rest] = React.Children.toArray(children);
  return (
    <>
      <div className="hero">
        <div className="hero-heading">
          {first}
        </div>

        <div className="hero-image">
          <img src="/images/hero-image.png" alt="Dewy overview" className="hero-image-overview" />
          <img src="/images/graph.gif" alt="Dewy graph" className="hero-image-graph" />
          <div className="version-animation-container">
            <VersionAnimation />
          </div>
        </div>

        <div className="hero-content">
          {rest}
        </div>
      </div>
      <style jsx>
      {`
        .hero-heading :global(h1) {
          font-size: 3rem;
          position: relative;
        }
        .hero-content {
          font-size: 1.5rem;
          padding-top: 1rem;
          padding-bottom: 1rem;
        }
        .hero-heading :global(h1):before {
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
        .hero-heading :global(h1):after {
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
        .hero-image {
          margin: 4rem 0 0;
          position: relative;
        }
        .hero-image-overview {
          display: block;
          margin: 0 auto;
          width: 80%;
          height: auto;
          position: relative;
        }
        .hero-image-graph {
          display: block;
          margin: 0 auto;
          width: 80%;
          height: auto;
          position: absolute;
          top: 40%;
          left: 0;
          width: 60%;
          margin-left: 20%;
          margin-right: 20%;
          height: auto;
          object-fit: cover;
        }
        .version-animation-container {
          margin: 0;
          padding: 0;
          max-width: 400px;
          position: absolute;
          top: 66%;
          left: 65%;
        }
        @media screen and (max-width: 600px) {
          .hero-heading :global(h1) {
            font-size: 2rem;
          }
          .hero-heading :global(h1):before {
            top: -1rem;
            left: -1.5rem;
            padding: .5rem;
            width: 25px;
            height: 25px;
          }
          .hero-heading :global(h1):after {
            bottom: -1rem;
            right: -1.5rem;
            padding: .5rem;
            width: 25px;
            height: 25px;
          }
          .hero-content {
            font-size: 1rem;
            padding-bottom: .5rem;
          }
          .hero-image-overview {
            width: 100%;
          }
        }
      `}
      </style>
    </>
  );
};