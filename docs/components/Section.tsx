import * as React from 'react';

export function Section({ children, className }) {
  return (
    <div className={['wrapper', className].filter(Boolean).join(' ')}>
      <section>
        {children}
      </section>
      <style jsx>
        {`
          .wrapper {
            width: 100%;
            padding: 5rem 0;
          }
          section {
            margin: 0 auto;
            padding: 0 2rem;
            max-width: 1200px;
          }
          .hero section {
            padding: 0 4rem;
            max-width: 900px;
          }
          .usecases section {
            padding: 0;
            max-width: 100%;
          }
          .core-benefits :global(.keyword) {
            justify-self: center;
          }
          .core-benefits section {
            max-width: 600px;
            margin: 0 auto;
          }
          .core-benefits :global(img) {
            width: 60%;
            height: auto;
            display: block;
            margin: 0 auto 3rem;
          }
          .sub-benefits {
            padding: 0 0 3rem;
          }
          .sub-benefits :global(.heading) {
            font-size: 2rem;
            font-weight: bold;
            margin: 0;
          }
          .sub-benefits :global(ul) {
            display: grid;
            grid-template-columns: repeat(2, 1fr);
            gap: 4rem;
            list-style: none;
            padding: 0;
            margin: 2rem 0;
          }
          .sub-benefits :global(.keyword) {
            justify-self: center;
          }
          .sub-benefits :global(img) {
            display: block;
            margin: 0 auto 3rem;
            width: 260px;
            height: auto;
          }
          .get-started {
            margin: 2rem 0;
            background-color: var(--accent-color);
            border-top: 1px solid var(--text-color);
            border-bottom: 1px solid var(--text-color);
          }
          .faq :global(.keyword) {
            text-align: center;
          }
          .faq :global(h2) {
            margin: 0;
            padding: 0 0 4rem;
            font-size: 2rem;
            text-align: center;
          }
          .faq :global(h3) {
            padding: 1rem 0 0;
            font-size: 1.2rem!important;
            font-weight: 900;
            color: var(--text-color);
          }
          .faq :global(.cards) :global(p) {
            color: var(--text-dim-color);
          }
          .faq :global(h3):before {
            content: "—";
            margin-right: 1rem;
          }
          .faq :global(.cards) {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
            gap: 3rem;
          }
          @media screen and (max-width: 1000px) {
            .sub-benefits :global(ul) {
              grid-template-columns: repeat(1, 1fr);
            }
            .sub-benefits :global(img) {
              width: 50%
            }
          }
          @media screen and (max-width: 600px) {
            .wrapper {
              padding: 2rem 0;
            }
            .hero section {
              padding: 0 2rem;
            }
          }
        `}
      </style>
    </div>
  );
}