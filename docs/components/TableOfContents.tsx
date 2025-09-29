import React, { useState, useEffect, useRef } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/router';
import { icons } from './Icons';

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
  const [isOpen, setIsOpen] = useState(false);
  const tocRef = useRef(null);

  const toggleToc = () => {
    setIsOpen(!isOpen);
  };

  const closeToc = () => {
    setIsOpen(false);
  };

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (tocRef.current && !tocRef.current.contains(event.target) && !event.target.closest('.toggler')) {
        closeToc();
      }
    };

    if (isOpen) {
      document.addEventListener('mousedown', handleClickOutside);
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [isOpen]);

  useEffect(() => {
    const handleRouteChange = () => {
      closeToc();
    };

    router.events.on('routeChangeStart', handleRouteChange);

    return () => {
      router.events.off('routeChangeStart', handleRouteChange);
    };
  }, [router]);

  const nestedItems = buildNestedItems(toc.filter((item) => item.id && (item.level === 2 || item.level === 3)));

  if (nestedItems.length <= 1) {
    return null;
  }

  return (
    <div>
      <nav ref={tocRef} className={`toc ${isOpen ? 'open' : ''}`}>
        <div className="toc-title">
          {icons('hamburger-left')}
          {title}
        </div>
        {renderItems(nestedItems)}
      </nav>
      <div className="toggler" onClick={toggleToc}>
        {icons('table-of-contents')}
      </div>
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
          .toc-title :global(svg) {
            vertical-align: middle;
            margin-right: .5rem;
            margin-top: -8px;
            width: 20px;
            height: 20px;
          }
          .toggler {
            position: fixed;
            right: 1.5rem;
            bottom: 1.5rem;
            border: 1px solid var(--text-color);
            padding: .3rem 1rem;
            border-radius: 30px;
            z-index: 20;
            display: none;
            backdrop-filter: blur(5px);
            cursor: pointer;
          }
          .toggler :global(svg) {
            width: 25px;
            height: 25px;
            vertical-align: middle;
            fill: var(--text-color);
            margin-top: -5px;
          }
          .toggler:hover {
            border: 1px solid var(--primary-color);
          }
          .toggler:hover :global(svg) {
            fill: var(--primary-color);
          }
          @media (max-width: 1400px) {
            nav {
              position: fixed;
              top: 0;
              right: 0;
              height: 100vh;
              width: 300px;
              max-height: none;
              backdrop-filter: blur(14px);
              z-index: 10;
              transform: translateX(100%);
              transition: transform 0.3s ease-in-out;
              overflow-y: auto;
              padding: 7rem 2rem 2rem 3rem;
            }
            nav.open {
              transform: translateX(0);
            }
            .toggler {
              display: block;
            }
          }
        `}
      </style>
    </div>
  );
}
