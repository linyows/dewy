import React from 'react';
import Link from 'next/link';
import { useRouter } from 'next/router';

export function TableOfContents({toc}) {
  const router = useRouter();
  const { pathname } = router;
  const title = pathname.startsWith('/ja') ? 'このページの内容' : 'On this page';
  const items = toc.filter(
    (item) => item.id && (item.level === 2 || item.level === 3)
  );

  if (items.length <= 1) {
    return null;
  }

  return (
    <nav className="toc">
      <div className='toc-title'>
        <svg xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round">
          <path d="M17 6.1H3"></path>
          <path d="M21 12.1H3"></path>
          <path d="M15.1 18H3"></path>
        </svg>
        {title}
      </div>
      <ul className="flex column">
        {items.map((item) => {
          const href = `#${item.id}`;
          const active =
            typeof window !== 'undefined' && window.location.hash === href;
          return (
            <li
              key={item.title}
              className={[
                active ? 'active' : undefined,
                item.level === 3 ? 'padded' : undefined,
              ]
                .filter(Boolean)
                .join(' ')}
            >
              <Link href={href}>
                {item.title}
              </Link>
            </li>
          );
        })}
      </ul>
      <style jsx>
        {`
          nav {
            position: sticky;
            top: calc(2.5rem + var(--top-nav-height));
            max-height: calc(100vh - var(--top-nav-height));
            flex: 0 0 auto;
            align-self: flex-start;
            margin-bottom: 1rem;
            padding: 1rem 0 0;
          }
          .toc-title {
            font-weight: bold;
            font-size: 1.1rem;
            margin-bottom: 1.2rem;
          }
          .toc-title svg {
            vertical-align: top;
            margin-right: 1rem;
            margin-top: 1px;
          }
          ul {
            margin: 0;
            padding: 0 0 0 2.2rem;
          }
          li {
            list-style-type: none;
          }
          li :global(a) {
            text-decoration: none;
          }
          li :global(a:hover),
          li.active :global(a) {
            text-decoration: underline;
          }
          li.padded {
            padding-left: 1rem;
          }
        `}
      </style>
    </nav>
  );
}
