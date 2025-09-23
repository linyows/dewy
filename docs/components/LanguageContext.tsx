import React, { createContext, useContext, useState, useEffect } from 'react';
import { useRouter } from 'next/router';

type Language = 'ja' | 'en';

interface LanguageContextType {
  language: Language;
  setLanguage: (lang: Language) => void;
  getUserPreferredLanguage: () => Language;
}

const LanguageContext = createContext<LanguageContextType | undefined>(undefined);

export function LanguageProvider({ children }: { children: React.ReactNode }) {
  const router = useRouter();
  const [language, setLanguageState] = useState<Language>('en');

  const getUserPreferredLanguage = (): Language => {
    // 1. Check user setting in localStorage
    if (typeof window !== 'undefined') {
      const userSetting = localStorage.getItem('dewy-language');
      if (userSetting === 'ja' || userSetting === 'en') {
        return userSetting;
      }
    }

    // 2. Check browser language
    if (typeof window !== 'undefined') {
      const browserLang = navigator.language || navigator.languages?.[0];
      if (browserLang?.startsWith('ja')) {
        return 'ja';
      }
    }

    // 3. Default to English
    return 'en';
  };

  const setLanguage = (lang: Language) => {
    if (typeof window !== 'undefined') {
      localStorage.setItem('dewy-language', lang);
    }
    setLanguageState(lang);

    // Navigate to the corresponding page in the selected language
    const currentPath = router.pathname;
    let newPath: string;

    if (lang === 'ja') {
      // Navigate to Japanese version
      if (currentPath.startsWith('/ja/')) {
        // Already on Japanese page
        return;
      } else if (currentPath === '/') {
        newPath = '/ja/introduction';
      } else {
        newPath = `/ja${currentPath}`;
      }
    } else {
      // Navigate to English version
      if (currentPath.startsWith('/ja/')) {
        // Remove /ja prefix
        const pathWithoutJa = currentPath.replace('/ja', '');
        newPath = pathWithoutJa === '' ? '/' : pathWithoutJa;
      } else {
        // Already on English page
        return;
      }
    }

    router.push(newPath);
  };

  useEffect(() => {
    // Initialize language based on current path and user preference
    const currentPath = router.pathname;

    if (currentPath.startsWith('/ja/')) {
      setLanguageState('ja');
    } else {
      const preferredLang = getUserPreferredLanguage();
      setLanguageState(preferredLang);

      // If user prefers Japanese but is on English page, redirect
      if (preferredLang === 'ja' && !currentPath.startsWith('/ja/')) {
        const newPath = currentPath === '/' ? '/ja/introduction' : `/ja${currentPath}`;
        router.replace(newPath);
      }
    }
  }, [router.pathname]);

  return (
    <LanguageContext.Provider value={{ language, setLanguage, getUserPreferredLanguage }}>
      {children}
    </LanguageContext.Provider>
  );
}

export function useLanguage() {
  const context = useContext(LanguageContext);
  if (context === undefined) {
    throw new Error('useLanguage must be used within a LanguageProvider');
  }
  return context;
}