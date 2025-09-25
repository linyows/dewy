import React from 'react';
import {useRouter} from 'next/router';
import Link from 'next/link';

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

  return (
    <div>
      <nav className={`sidenav ${className}`}>
        {items.map((item) => (
          <div key={item.title}>
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
      <div className="toggler">
        Sidebar
      </div>
      <style jsx>
        {`
          nav {
            position: sticky;
            top: var(--top-nav-height);
            max-height: var(--top-nav-height);
            flex: 0 0 auto;
            padding: 3rem 2.5rem 2.5rem var(--side-width);
          }
          span {
            font-weight: bold;
            padding: 0.5rem 0 0.5rem 1.6rem;
            poisition: relative;
          }
          span:before {
            content: '⏺';
            display: block;
            position: absolute;
            font-size: 1.4rem;
            line-height: 1.4rem;
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
          @media (max-width: 1240px) {
            nav {
              display: none;
            }
          }
          .toggler {
            position: fixed;
            right: 2rem;
            bottom: 2rem;
            border: 1px solid var(--border-color);
            padding: .3rem 1rem;
            border-radius: 4px;
            background: #fff;
            z-index: 10;
            box-shadow: 5px 5px 1px rgba(0, 0, 0, 0.15);
          }
        `}
      </style>
    </div>
  );
}
