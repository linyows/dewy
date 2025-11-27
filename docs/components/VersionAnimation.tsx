import * as React from 'react';
import { useState, useEffect } from 'react';

interface VersionAnimationProps {
  className?: string;
  children?: React.ReactNode;
}

export function VersionAnimation({ className, children }: VersionAnimationProps) {
  const versions = ['1.2.3', '1.2.4', '1.2.5', '1.3.0', '1.3.1', '2.0.0'];
  const [currentIndex, setCurrentIndex] = useState(0);
  const [showLogs, setShowLogs] = useState([false, false, false]);

  const currentVersion = versions[currentIndex];
  const nextVersion = versions[(currentIndex + 1) % versions.length];

  useEffect(() => {
    // Reset logs and update version
    setShowLogs([false, false, false]);

    // Show logs sequentially
    const timer1 = setTimeout(() => setShowLogs([true, false, false]), 100);
    const timer2 = setTimeout(() => setShowLogs([true, true, false]), 600);
    const timer3 = setTimeout(() => setShowLogs([true, true, true]), 1200);

    // Move to next version
    const versionTimer = setTimeout(() => {
      setCurrentIndex((prev) => (prev + 1) % versions.length);
    }, 2000);

    return () => {
      clearTimeout(timer1);
      clearTimeout(timer2);
      clearTimeout(timer3);
      clearTimeout(versionTimer);
    };
  }, [currentIndex, versions.length]);

  return (
    <>
      <div className={`version-animation ${className || ''}`}>
        {children}
        <div className="version">v{currentVersion}</div>
        <div className="logs">
          <div className={`log ${showLogs[0] ? 'visible' : ''}`}>
            Checking for updates...
          </div>
          <div className={`log ${showLogs[1] ? 'visible' : ''}`}>
            New version found: v{nextVersion}
          </div>
          <div className={`log success ${showLogs[2] ? 'visible' : ''}`}>
            Deployed ✓
          </div>
        </div>
      </div>
      <style jsx>
        {`
          .version-animation {
            margin: 0;
            padding: 0;
            width: 100%;
            height: auto;
          }
          .version {
            font-family: 'Courier New', Courier, monospace;
            font-size: 3rem;
            font-weight: bold;
            color: var(--text-color);
            margin-bottom: .5rem;
            transition: opacity 0.3s ease;
            white-space: nowrap;
          }
          .logs {
            font-family: 'Courier New', Courier, monospace;
            font-size: 1rem;
            text-align: left;
            max-width: 400px;
            margin: 0;
          }
          .log {
            opacity: 0;
            transform: translateY(10px);
            transition: opacity 0.3s ease, transform 0.3s ease;
            padding: 0.25rem 0;
          }
          .log.visible {
            opacity: 1;
            transform: translateY(0);
          }
          .log.success {
            color: var(--tip-color);
            font-weight: 900;
          }
          @media screen and (max-width: 900px) {
            .version {
              font-size: 2rem;
            }
            .logs {
              font-size: .8rem;
            }
          }
          @media screen and (max-width: 600px) {
            .version-animation {
              margin-top: -1rem;
            }
            .version {
              font-size: 1.2rem;
              margin-bottom: .2rem;
            }
            .logs {
              letter-spacing: -1px;
            }
          }
        `}
      </style>
    </>
  );
};
