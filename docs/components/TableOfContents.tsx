import React from 'react';
import Link from 'next/link';
import { useRouter } from 'next/router';

// Function to build a nested structure from flat items
const buildNestedItems = (items) => {
  const nested = [];
  const stack = [];

  items.forEach((item) => {
    while (stack.length > 0 && stack[stack.length - 1].level >= item.level) {
      stack.pop();
    }

    const parent = stack.length > 0 ? stack[stack.length - 1] : null;

    const newItem = { ...item, children: [] };
    if (parent) {
      parent.children.push(newItem);
    } else {
      nested.push(newItem);
    }

    stack.push(newItem);
  });

  return nested;
};

const renderItems = (items) => (
  <ul>
    {items.map((item) => (
      <li key={item.id}>
        <Link href={`#${item.id}`}>
          {item.title}
        </Link>
        {item.children.length > 0 && renderItems(item.children)}
      </li>
    ))}
    <style jsx>
      {`
        ul {
          margin: 0;
          padding: .25rem 0 .25rem 2rem;
          font-size: 0.9rem;
        }
        li :global(a) {
          text-decoration: none;
          padding: .2rem 0;
        }
        li :global(a:hover),
        li.active :global(a) {
          text-decoration: underline;
        }
      `}
    </style>
  </ul>
);

export function TableOfContents({ toc }) {
  const router = useRouter();
  const { pathname } = router;
  const title = pathname.startsWith('/ja') ? 'このページの内容' : 'On this page';

  const nestedItems = buildNestedItems(toc.filter((item) => item.id && (item.level === 2 || item.level === 3)));

  if (nestedItems.length <= 1) {
    return null;
  }

  return (
    <nav className="toc">
      <div className="toc-title">
        <svg
          xmlns="http://www.w3.org/2000/svg"
          width="20"
          height="20"
          viewBox="0 0 20 20"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          strokeLinecap="round"
          strokeLinejoin="round"
        >
          <path d="M17 6.1H3"></path>
          <path d="M21 12.1H3"></path>
          <path d="M15.1 18H3"></path>
        </svg>
        {title}
      </div>
      {renderItems(nestedItems)}
      <style jsx>
        {`
          nav {
            position: sticky;
            top: calc(2.5rem + var(--top-nav-height));
            max-height: calc(100vh - var(--top-nav-height));
            flex: 0 0 auto;
            align-self: flex-start;
            margin-bottom: 1rem;
            padding: 1rem var(--side-width) 0 0;
          }
          .toc-title {
            font-weight: bold;
            font-size: 1.1rem;
            margin-bottom: 1rem;
          }
          .toc-title svg {
            vertical-align: top;
            margin-right: .5rem;
            margin-top: 1px;
          }
          @media (max-width: 1400px) {
            nav {
              display: none;
            }
          }
        `}
      </style>
    </nav>
  );
}
