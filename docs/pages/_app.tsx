import React from 'react';
import Head from 'next/head';
import { Inter, Zen_Kaku_Gothic_New } from "next/font/google";
import { useRouter } from 'next/router';
import { SideNav, TableOfContents, TopNav } from '../components';
import MarkdocTemplate from '../components/MarkdocTemplate';
import { LanguageProvider } from '../components/LanguageContext';

import 'prismjs';
// Import other Prism themes here
import 'prismjs/components/prism-bash.min';
import 'prismjs/components/prism-go.min';
import 'prismjs/components/prism-systemd.min';
import 'prismjs/themes/prism.css';

import '../public/globals.css'

import type { AppProps } from 'next/app'
import type { MarkdocNextJsPageProps } from '@markdoc/next.js'

const TITLE = 'Dewy';
const DESCRIPTION = 'Dewy enables declarative deployment of applications in non-Kubernetes environments.';

const inter = Inter({ subsets: ["latin"] });
const zenKakuGothicNew = Zen_Kaku_Gothic_New({ subsets: ["latin"], weight: ["500", "900"] });

function collectHeadings(node, sections = []) {
  if (node) {
    if (node.name === 'Heading') {
      const title = node.children[0];

      if (typeof title === 'string') {
        sections.push({
          ...node.attributes,
          title
        });
      }
    }

    if (node.children) {
      for (const child of node.children) {
        collectHeadings(child, sections);
      }
    }
  }

  return sections;
}

export type MyAppProps = MarkdocNextJsPageProps

export default function MyApp({ Component, pageProps }: AppProps<MyAppProps>) {
  const { markdoc } = pageProps;
  const router = useRouter();
  const { pathname } = router;

  let title = TITLE;
  let description = DESCRIPTION;

  if (markdoc) {
    if (markdoc.frontmatter.title) {
      title = markdoc.frontmatter.title;
    }
    if (markdoc.frontmatter.description) {
      description = markdoc.frontmatter.description;
    }
  }

  // Use Zen Kaku Gothic New for /ja/** paths, otherwise use Inter
  const font = pathname.startsWith('/ja') ? zenKakuGothicNew : inter;

  // Table of Contents
  const toc = markdoc?.content ? collectHeadings(pageProps.markdoc.content) : [];

  // Dynamically construct the file path for the "Edit this page on GitHub" link
  const filePath = `docs/pages${pathname === '/' ? '/index' : pathname === '/ja' ? '/ja/index' : pathname}.md`;

  // Whether the current page is the index page
  const isDocs = pathname !== '/' && pathname !== '/ja';

  return (
    <LanguageProvider>
      <Head>
        <title>{title}</title>
        <meta name="viewport" content="width=device-width, initial-scale=1.0" />
        <meta name="referrer" content="strict-origin" />
        <meta name="title" content={title} />
        <meta name="description" content={description} />
        <link rel="shortcut icon" href="/favicon.ico" />
        <link rel="icon" href="/favicon.ico" />
        <meta property="og:type" content="website" />
        <meta property="og:url" content="https://dewy.linyo.ws" />
        <meta property="og:title" content={title} />
        <meta property="og:description" content={description} />
        <meta property="og:image" content="https://dewy.linyo.ws/images/share.png" />
        <meta name="twitter:card" content="summary" />
        <meta name="twitter:image" content="https://dewy.linyo.ws/images/share.png" />
      </Head>
      <TopNav className={font.className} />
      <div className={`page ${font.className}`}>
        {isDocs && <SideNav className={font.className} />}
        <div className={`${isDocs ? 'main-and-toc' : 'landing'}`}>
          <main className={`${isDocs ? 'docs flex column' : ''}`}>
            <MarkdocTemplate content={<Component {...pageProps} />} filePath={filePath} />
          </main>
          {isDocs && <TableOfContents toc={toc} />}
        </div>
      </div>
      <footer>
        <p>© 2018-{new Date().getFullYear()} <a href="https://github.com/linyows" target="_blank" rel="noopener noreferrer">linyows</a></p>
      </footer>
      <style jsx>
        {`
          .page {
            top: var(--top-nav-height);
            display: flex;
            width: 100vw;
            flex-grow: 1;
          }
          .landing {
            margin: 0;
            width: 100%;
            padding: 10vh 0 0;
          }
          .main-and-toc {
            max-width: 1800px;
            margin: 0 auto;
            flex-grow: 1;
            display: grid;
            grid-template-columns: minmax(0, 1fr) 400px;
            gap: 2rem;
          }
          .docs:before {
            content: "";
            position: absolute;
            top: var(--top-nav-height);
            left: -1rem;
            padding: 1rem;
            border-top: 1px solid var(--text-color);
            border-left: 1px solid var(--text-color);
            display: block;
            width: 50px;
            height: 50px;
            z-index: -1;
          }
          .docs {
            flex-grow: 1;
            font-size: 16px;
            padding: var(--top-nav-height) 2rem 2rem;
            position: relative;
          }
          footer {
            text-align: center;
            padding: 2rem 1rem;
            font-size: 0.9rem;
            color: var(--text-dim-color);
          }
          @media (max-width: 1400px) {
            .main-and-toc {
              display: block;
              padding-right: 2rem;
            }
          }
          @media (max-width: 1240px) {
            .main-and-toc {
              padding-right: 0;
              width: 100%;
            }
            main:before {
              display: none;
            }
            main {
              display: block;
              position: relative;
            }
          }
        `}
      </style>
    </LanguageProvider>
  );
}
