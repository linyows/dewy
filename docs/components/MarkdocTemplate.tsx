import React from 'react';

interface MarkdocTemplateProps {
  content: React.ReactNode;
  filePath: string; // Add filePath prop
}

export default function MarkdocTemplate({ content, filePath }: MarkdocTemplateProps) {
  const githubEditUrl = `https://github.com/linyows/dewy/edit/main/${filePath}`;

  return (
    <div>
      {content}
      <div className="edit-link">
        <a href={githubEditUrl} target="_blank" rel="noopener noreferrer">
          Edit this page on GitHub
        </a>
      </div>
      <style jsx>
        {`
          .edit-link {
            margin: 3rem 0;
            text-align: left;
          }
          .edit-link a {
            text-decoration: underline;
          }
          .edit-link a:hover {
            text-decoration: none;
          }
        `}
      </style>
    </div>
  );
}
