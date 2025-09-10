import React from 'react';
import Head from 'next/head';
import { Inter, Zen_Kaku_Gothic_New } from "next/font/google";
import { useRouter } from 'next/router';
import { SideNav, TableOfContents, TopNav } from '../components';
import MarkdocTemplate from '../components/MarkdocTemplate';

import 'prismjs';
// Import other Prism themes here
import 'prismjs/components/prism-bash.min';
import 'prismjs/themes/prism.css';

import '../public/globals.css'

import type { AppProps } from 'next/app'
import type { MarkdocNextJsPageProps } from '@markdoc/next.js'

const TITLE = 'Markdoc';
const DESCRIPTION = 'A powerful, flexible, Markdown-based authoring framework';

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

  const toc = pageProps.markdoc?.content
    ? collectHeadings(pageProps.markdoc.content)
    : [];

  // Dynamically construct the file path for the "Edit this page on GitHub" link
  const filePath = `docs/pages${pathname === '/' ? '/index' : pathname}.md`;

  return (
    <>
      <Head>
        <title>{title}</title>
        <meta name="viewport" content="width=device-width, initial-scale=1.0" />
        <meta name="referrer" content="strict-origin" />
        <meta name="title" content={title} />
        <meta name="description" content={description} />
        <link rel="shortcut icon" href="/favicon.ico" />
        <link rel="icon" href="/favicon.ico" />
      </Head>
      <TopNav className={font.className} />
      <div className={`page ${font.className}`}>
        <SideNav className={font.className} />
        <div className='main-and-toc'>
          <main className="flex column">
            <MarkdocTemplate content={<Component {...pageProps} />} filePath={filePath} />
          </main>
          <TableOfContents toc={toc} />
        </div>
      </div>
      <footer>
        <p>© 2018-{new Date().getFullYear()} linyows</p>
      </footer>
      <style jsx>
        {`
          .page {
            top: var(--top-nav-height);
            display: flex;
            width: 100vw;
            flex-grow: 1;
          }
          .main-and-toc {
            max-width: 1800px;
            margin: 0 auto;
            flex-grow: 1;
            display: grid;
            grid-template-columns: minmax(0, 1fr) 400px;
            gap: 2rem;
          }
          main:before {
            content: "┌─────────\A│\A│\A│\A│\A│\A│\A│";
            white-space: pre-wrap;
            line-height: 1;
            position: absolute;
            display: inline-block;
            top: calc(var(--top-nav-height) - 1.7rem);
            left: -3rem;
            padding: 1rem;
          }
          main {
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
        `}
      </style>
    </>
  );
}
