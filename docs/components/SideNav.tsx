import React, { useState, useEffect, useRef } from 'react';
import {useRouter} from 'next/router';
import Link from 'next/link';
import { icons } from './Icons';

const jaItems = [
  {
    title: '概要',
    links: [
      { href: '/ja/introduction', children: 'はじめに' },
      { href: '/ja/getting-started', children: '使ってみよう' },
      { href: '/ja/installation', children: 'インストール' },
      { href: '/ja/faq', children: 'よくある質問' },
      { href: '/ja/contributing', children: 'コントリビューション' },
    ],
  },
  {
    title: 'コンセプト',
    links: [
      { href: '/ja/architecture', children: 'アーキテクチャ' },
      { href: '/ja/registry', children: 'レジストリー' },
      { href: '/ja/artifact', children: 'アーティファクト' },
      { href: '/ja/notifier', children: '通知' },
      { href: '/ja/cache', children: 'キャッシュ' },
      { href: '/ja/versioning', children: 'バージョニング' },
      { href: '/ja/deployment-hooks', children: 'デプロイフック' },
    ],
  },
  {
    title: '運用',
    links: [
      { href: '/ja/signal-handling', children: 'シグナルハンドリング' },
      { href: '/ja/multi-port', children: 'マルチポート' },
      { href: '/ja/structured-logging', children: '構造化ログ' },
      { href: '/ja/cache-configuration', children: 'キャッシュ設定' },
      { href: '/ja/deployment-workflow', children: 'デプロイメントワークフロー' },
    ],
  },
];

const enItems = [
  {
    title: 'Overview',
    links: [
      { href: '/introduction', children: 'Introduction' },
      { href: '/getting-started', children: 'Getting Started' },
      { href: '/installation', children: 'Installation' },
      { href: '/faq', children: 'FAQ' },
      { href: '/contributing', children: 'Contributing' },
    ],
  },
  {
    title: 'Concepts',
    links: [
      { href: '/architecture', children: 'Architecture' },
      { href: '/registry', children: 'Registry' },
      { href: '/artifact', children: 'Artifact' },
      { href: '/notifier', children: 'Notifier' },
      { href: '/cache', children: 'Cache' },
      { href: '/versioning', children: 'Versioning' },
      { href: '/deployment-hooks', children: 'Deployment Hooks' },
    ],
  },
  {
    title: 'Operations',
    links: [
      { href: '/signal-handling', children: 'Signal Handling' },
      { href: '/multi-port', children: 'Multi-Port' },
      { href: '/structured-logging', children: 'Structured Logging' },
      { href: '/cache-configuration', children: 'Cache Configuration' },
      { href: '/deployment-workflow', children: 'Deployment Workflow' },
    ],
  },
];

export function SideNav({ className }) {
  const router = useRouter();
  const { pathname } = router;
  const items = pathname.startsWith('/ja') ? jaItems : enItems;
  const [isOpen, setIsOpen] = useState(false);
  const navRef = useRef(null);

  const toggleNav = () => {
    setIsOpen(!isOpen);
  };

  const closeNav = () => {
    setIsOpen(false);
  };

  useEffect(() => {
    const handleClickOutside = (event) => {
      if (navRef.current && !navRef.current.contains(event.target) && !event.target.closest('.toggler')) {
        closeNav();
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
      closeNav();
    };

    router.events.on('routeChangeStart', handleRouteChange);

    return () => {
      router.events.off('routeChangeStart', handleRouteChange);
    };
  }, [router]);

  return (
    <div>
      <nav ref={navRef} className={`sidenav ${className} ${isOpen ? 'open' : ''}`}>
        {items.map((item) => (
          <div className="itemdiv" key={item.title}>
            <span>{item.title}</span>
            <ul className="flex column">
              {item.links.map((link) => {
                const active = router.pathname === link.href;
                return (
                  <li key={link.href} className={active ? 'active' : ''}>
                    <Link {...link} />
                  </li>
                );
              })}
            </ul>
          </div>
        ))}
      </nav>
      <div className="toggler" onClick={toggleNav}>
        {icons('sidebar')}
      </div>
      <style jsx>
        {`
          nav {
            position: sticky;
            top: var(--top-nav-height);
            max-height: var(--top-nav-height);
            flex: 0 0 auto;
            padding: 3rem 2.5rem 2.5rem var(--side-width);
            white-space: nowrap;
          }
          .itemdiv {
            position: relative;
          }
          span {
            font-weight: bold;
            padding: 0.5rem 0 0.5rem 1.6rem;
            poisition: relative;
          }
          span:before {
            content: '⚫︎';
            display: block;
            position: absolute;
            top: .2rem;
            left: -0.2rem;
            font-size: 1.2rem;
            line-height: 1.2rem;
            color: var(--primary-color);
          }
          ul {
            padding: 0 0 1.5rem 1.7rem;
            position: relative;
          }
          ul:before {
            content: '⎿';
            position: absolute;
            left: -.1rem;
            top: 0;
            bottom: 0;
            width: 1px;
          }
          li {
            list-style: none;
            margin: 0;
            padding: .4rem 0;
          }
          li :global(a) {
            text-decoration: none;
          }
          li :global(a:hover),
          li.active :global(a) {
            text-decoration: underline;
          }
          .toggler {
            position: fixed;
            left: 1.5rem;
            bottom: 1.5rem;
            border: 1px solid var(--text-color);
            padding: .3rem 1rem;
            border-radius: 30px;
            z-index: 20;
            display: none;
            backdrop-filter: blur(5px);
            cursor: pointer;
          }
          .toggler:hover {
            border: 1px solid var(--primary-color);
          }
          .toggler :global(svg) {
            width: 30px;
            height: 30px;
            vertical-align: middle;
            fill: var(--text-color);
          }
          .toggler:hover :global(svg) {
            fill: var(--primary-color);
          }
          @media (max-width: 1240px) {
            nav {
              position: fixed;
              top: 0;
              left: 0;
              height: 100vh;
              width: 300px;
              max-height: none;
              backdrop-filter: blur(14px);
              z-index: 10;
              transform: translateX(-100%);
              transition: transform 0.3s ease-in-out;
              overflow-y: auto;
              padding: 7rem 3rem 2rem;
            }
            nav.open {
              transform: translateX(0);
            }
            .toggler {
              display: block;
            }
          }
          @media (max-width: 900px) {
            span:before {
              top: 0;
              left: 0;
              font-size: 1.4rem;
              line-height: 1.4rem;
            }
          }
        `}
      </style>
    </div>
  );
}
