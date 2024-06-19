import type { GetStaticProps, InferGetStaticPropsType } from 'next'
import { useEffect, useState } from 'react'
import Head from 'next/head'
import Link from 'next/link'
import {
  FetchBlocks,
  FetchPage,
  FetchBlocksRes,
} from 'rotion'
import { Page, Link as RotionLink } from 'rotion/ui'
import styles from '@/styles/Home.module.css'

type Props = {
  icon: string
  logo: string
  blocks: FetchBlocksRes
}

export const getStaticProps: GetStaticProps<Props> = async (context) => {
  const id = process.env.HOMEPAGE_ID as string
  const page = await FetchPage({ page_id: id, last_edited_time: 'force' })
  const logo = page.cover?.src || ''
  const icon = page.icon!.src
  const blocks = await FetchBlocks({ block_id: id, last_edited_time: page.last_edited_time })

  return {
    props: {
      icon,
      logo,
      blocks,
    }
  }
}

const InlineSVG = ({ src }: { src: string }) => {
  const [svgContent, setSvgContent] = useState('')

  useEffect(() => {
    fetch(src)
      .then(response => response.text())
      .then(data => setSvgContent(data))
  }, [src])

  return <div dangerouslySetInnerHTML={{ __html: svgContent }} style={{ display: 'inline-block' }} />
}

export default function Home({ logo, icon, blocks }: InferGetStaticPropsType<typeof getStaticProps>) {
  const y = new Date(Date.now()).getFullYear()
  return (
    <>
      <Head>
        <title>Dewy</title>
        <link rel="icon" type="image/svg+xml" href={icon} />
      </Head>
      <div className={styles.layout}>
        <div className={styles.nav}>
          <header className={styles.header}>
            <div className={styles.icon}>
              <InlineSVG src={icon} />
            </div>
            <div className={styles.logo}>
              <h1> <InlineSVG src={logo} /> </h1>
            </div>
          </header>
        </div>

        <div className={styles.box}>
          <div className={styles.page}>
            <Page blocks={blocks} href="/[title]" link={Link as RotionLink} />
          </div>
        </div>

        <footer className={styles.footer}>
          &copy; {y} <a href="https://github.com/linyows" target="_blank" rel="noreferrer">linyows</a>
          {` `} | {` `}
          <a href="https://github.com/linyows/dewy/issues" target="_blank" rel="noreferrer">Github Issues</a>
        </footer>
      </div>
    </>
  )
}
