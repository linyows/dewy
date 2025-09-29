import React, { useState, useRef, useEffect } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/router';
import { useLanguage } from './LanguageContext';
import { icons } from './Icons';

const jaNav = {
  "guide": { href: "/ja/introduction/", label: "ガイド" },
  "docs": { href: "/ja/architecture/", label: "ドキュメント" },
  "reference": { href: "/ja/reference", label: "リファレンス" },
};

const enNav = {
  "guide": { href: "/introduction/", label: "Guide" },
  "docs": { href: "/architecture/", label: "Docs" },
  "reference": { href: "/reference", label: "Reference" },
};

export function TopNav({ className }) {
  const router = useRouter();
  const { pathname } = router;
  const { language, setLanguage } = useLanguage();
  const [isLanguageMenuOpen, setIsLanguageMenuOpen] = useState(false);
  const [isNavMenuOpen, setIsNavMenuOpen] = useState(false);
  const languageMenuRef = useRef<HTMLDivElement>(null);
  const navMenuRef = useRef<HTMLElement>(null);

  const nav = pathname.startsWith('/ja') ? jaNav : enNav;
  const currentLangLabel = language === 'ja' ? '日本語' : 'English';

  // Close menu when clicking outside
  useEffect(() => {
    function handleClickOutside(event: MouseEvent) {
      if (languageMenuRef.current && !languageMenuRef.current.contains(event.target as Node)) {
        setIsLanguageMenuOpen(false);
      }
      if (navMenuRef.current && !navMenuRef.current.contains(event.target as Node) && !(event.target as Element).closest('.hamburger-menu')) {
        setIsNavMenuOpen(false);
      }
    }

    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, []);

  // Close nav menu on route change
  useEffect(() => {
    const handleRouteChange = () => {
      closeNavMenu();
    };

    router.events.on('routeChangeStart', handleRouteChange);

    return () => {
      router.events.off('routeChangeStart', handleRouteChange);
    };
  }, [router]);

  const handleLanguageChange = (lang: 'ja' | 'en') => {
    setLanguage(lang);
    setIsLanguageMenuOpen(false);
  };

  const toggleNavMenu = () => {
    setIsNavMenuOpen(!isNavMenuOpen);
  };

  const closeNavMenu = () => {
    setIsNavMenuOpen(false);
  };

  return (
    <nav className={className}>
      <div className="logo">
        <Link href="/" className="flex">
          <span className="logo-icon">{icons('dewy')}</span>
          <span className="logo-font">{icons('dewy-font')}</span>
        </Link>
      </div>
      <section>
        {Object.entries(nav).map(([key, { label, href }]) => (
          <div key={key}>
            <Link key={href} href={href} className="nav-link">
              <span className="nav-link-icon">
                {icons(key)}
              </span>
              {label}
            </Link>
          </div>
        ))}

        <a className="github" href="https://github.com/linyows/dewy" target="_blank" rel="noopener noreferrer" aria-label="GitHub">
          {icons('github')}
        </a>

        <div className="language-selector" ref={languageMenuRef}>
          <button
            className="language-button"
            type="button"
            aria-haspopup="menu"
            aria-expanded={isLanguageMenuOpen}
            onClick={() => setIsLanguageMenuOpen(!isLanguageMenuOpen)}
          >
            <span className="lang">
              {icons('lang')}
            </span>
            <p>{currentLangLabel}</p>
            <svg className={`toggle ${isLanguageMenuOpen ? 'open' : ''}`} xmlns="http://www.w3.org/2000/svg" width="20" height="20" viewBox="0 0 20 20" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <path d="m6 9 6 6 6-6"></path>
            </svg>
          </button>

          {isLanguageMenuOpen && (
            <div className="language-menu">
              {language === 'ja' ? (
                <button className="language-option" onClick={() => handleLanguageChange('en')} >
                  English
                </button>
              ) : (
                <button className="language-option" onClick={() => handleLanguageChange('ja')} >
                  日本語
                </button>
              )}
            </div>
          )}
        </div>
      </section>

      <section ref={navMenuRef} className={`mobile-nav ${isNavMenuOpen ? 'open' : ''}`}>
        {Object.entries(nav).map(([key, { label, href }]) => (
          <div key={key}>
            <Link key={href} href={href} className="mobile-nav-link" onClick={closeNavMenu}>
              <span className="nav-link-icon">
                {icons(key)}
              </span>
              {label}
            </Link>
          </div>
        ))}

        <a className="github-mobile" href="https://github.com/linyows/dewy" target="_blank" rel="noopener noreferrer" aria-label="GitHub">
          {icons('github')}
          <span>GitHub</span>
        </a>

        <div className="language-selector-mobile">
          {language === 'ja' ? (
            <button className="language-option-mobile" onClick={() => handleLanguageChange('en')} >
              {icons('lang')}
              <span>English</span>
            </button>
          ) : (
            <button className="language-option-mobile" onClick={() => handleLanguageChange('ja')} >
              {icons('lang')}
              <span>日本語</span>
            </button>
          )}
        </div>
      </section>

      <div className="hamburger-menu" onClick={toggleNavMenu}>
        {icons('hamburger')}
      </div>
      <style jsx>
        {`
          .logo-icon {
            display: inline-block;
            border-radius: 8px;
            background: var(--primary-color);
            fill: #fff;
            width: 40px;
            height: 40px;
            overflow: hidden;
            padding: 10px;
          }
          .logo-font {
            display: inline-block;
            width: 100px;
            height: 36px;
            margin-left: 20px;
            vertical-align: top;
            fill: var(--primary-color);
            margin-top: 2px;
          }
          .logo {
            flex-shrink: 0;
          }
          nav {
            top: 0;
            position: fixed;
            display: grid;
            grid-template-columns: 220px minmax(0, 1fr);
            gap: 2rem;
            width: 100%;
            z-index: 100;
            align-items: center;
            gap: 1rem;
            padding: 1.5rem 2.5rem;
            backdrop-filter: blur(5px);
          }
          nav :global(a) {
            text-decoration: none;
          }
          section {
            display: flex;
            gap: 2rem;
            padding: 0;
            flex-grow: 1;
            justify-content: flex-end;
          }
          .language-selector {
            position: relative;
            margin-top: -0.2rem;
          }
          .language-button {
            background: none;
            padding: .2rem .6rem;
            border-radius: 4px;
            border: 1px solid rgba(23, 23, 22, 0.4);
            cursor: pointer;
            display: flex;
            align-items: center;
            line-height: 1;
            outline: 2px solid transparent;
            outline-offset: 2px;
            box-shadow: 0 0 #0000, 0 0 #0000, 0 0 #0000;
            background-image: linear-gradient(to bottom, #fff, #f9f9f9);
          }
          .language-button p {
            margin: 0;
          }
          .language-menu {
            position: absolute;
            top: 100%;
            right: 0;
            margin-top: 0.5rem;
            border: 1px solid var(--border-color);
            border-radius: 4px;
            box-shadow: 5px 5px 1px rgba(0, 0, 0, 0.15);
            min-width: 110px;
            z-index: 1000;
          }
          .language-option {
            width: 100%;
            padding: 0.5rem 0.75rem;
            border: none;
            background: none;
            cursor: pointer;
            display: flex;
            align-items: center;
            font-size: 0.85rem;
            text-align: left;
            border-radius: 4px;
            margin: 0 0.25rem;
            width: calc(100% - 0.5rem);
          }
          .language-option:hover {
            background-color: rgba(23, 23, 22, 0.05);
          }
          .lang :global(svg) {
            width: 1.4rem;
            height: 1.4rem;
            fill: var(--primary-color);
            margin: 0 .4rem 0 0;
          }
          .toggle {
            width: .8rem;
            margin-left: .4rem;
            vertical-align: bottom;
            transition: transform 0.2s ease;
          }
          .toggle.open {
            transform: rotate(180deg);
          }
          .github :global(svg) {
            width: 1.6rem;
            height: 1.6rem;
            fill: var(--text-color);
            margin-left: 1rem;
            cursor: pointer;
          }
          .nav-link-icon {
            width: 20px;
            height: 20px;
            fill: var(--primary-color);
            display: inline-block;
            vertical-align: top;
            margin: 2px .5rem 0 0;
          }
          .nav-link a {
            color: var(--primary-color);
          }
          .hamburger-menu {
            position: absolute;
            top: 2rem;
            right: 2rem;
            display: none;
            width: 26px;
            height: 26px;
            cursor: pointer;
            z-index: 110;
          }
          .mobile-nav {
            position: fixed;
            top: 0;
            left: 0;
            width: 100%;
            background: var(--bg-color, #fff);
            border-bottom: 1px solid var(--border-color);
            z-index: -1;
            padding: calc(var(--top-nav-height) + 2rem) 2rem 2rem;
            transform: translateY(-100%);
            transition: transform 0.3s ease-in-out;
            display: none;
            flex-direction: column;
            gap: 1.5rem;
          }
          .mobile-nav.open {
            transform: translateY(0);
          }
          .mobile-nav-link {
            display: flex;
            align-items: center;
            padding: 1rem 0;
            font-size: 1.1rem;
            font-weight: 500;
            border-bottom: 1px solid rgba(0,0,0,0.1);
          }
          .mobile-nav-link .nav-link-icon {
            margin-right: 1rem;
          }
          .github-mobile {
            display: flex;
            align-items: center;
            padding: 1rem 0;
            font-size: 1.1rem;
            font-weight: 500;
            border-top: 1px solid rgba(0,0,0,0.1);
            border-bottom: 1px solid rgba(0,0,0,0.1);
          }
          .github-mobile :global(svg) {
            width: 1.6rem;
            height: 1.6rem;
            fill: var(--text-color);
            margin-right: 1rem;
          }
          .language-selector-mobile {
            margin-top: 1rem;
          }
          .language-option-mobile {
            display: flex;
            align-items: center;
            padding: 0;
            font-size: 1.1rem;
            font-weight: 500;
            background: none;
            border: none;
            cursor: pointer;
            width: 100%;
            text-align: left;
            margin-top: -1.2rem;
          }
          .language-selector-mobile span {
            color: var(--text-color);
          }
          .language-option-mobile :global(svg) {
            width: 1.4rem;
            height: 1.4rem;
            fill: var(--primary-color);
            margin-right: 1rem;
          }
          @media (max-width: 1200px) {
            nav {
              padding-left: 2rem;
              padding-right: 2rem;
              backdrop-filter: blur(7px);
            }
          }
          @media (max-width: 900px) {
            section:first-of-type {
              display: none;
            }
            .hamburger-menu {
              display: block;
            }
            .mobile-nav {
              display: flex;
            }
          }
          @media (max-width: 460px) {
            .logo-icon {
              width: 32px;
              height: 32px;
              padding: 6px;
            }
            .logo-font {
              width: 80px;
              height: 32px;
              margin-left: 14px;
              margin-top: 0px;
            }
            .hamburger-menu {
              top: 1.8rem;
              right: 1.8rem;
              width: 24px;
              height: 24px;
            }
          }
        `}
      </style>
    </nav>
  );
}
