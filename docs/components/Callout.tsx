import * as React from 'react';

const icon = (name: string) => {
  switch (name) {
    case 'warning':
      return (
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
          <path d="m23.121,6.151L17.849.878c-.567-.566-1.321-.878-2.121-.878h-7.455c-.8,0-1.554.312-2.122.879L.879,6.151c-.566.567-.879,1.32-.879,2.121v7.456c0,.801.312,1.554.879,2.121l5.272,5.273c.567.566,1.321.878,2.121.878h7.455c.8,0,1.554-.312,2.122-.879l5.271-5.272c.566-.567.879-1.32.879-2.121v-7.456c0-.801-.313-1.554-.879-2.121Zm-1.121,9.577c0,.263-.106.521-.293.707l-5.271,5.271c-.19.189-.442.294-.709.294h-7.455c-.267,0-.519-.104-.708-.293l-5.271-5.272c-.187-.187-.293-.444-.293-.707v-7.456c0-.263.106-.521.293-.707L7.563,2.294c.19-.189.442-.294.709-.294h7.455c.267,0,.519.104.708.293l5.271,5.272c.187.187.293.444.293.707v7.456Zm-9-2.728h-2v-7h2v7Zm.5,3.5c0,.828-.672,1.5-1.5,1.5s-1.5-.672-1.5-1.5.672-1.5,1.5-1.5,1.5.672,1.5,1.5Z"/>
        </svg>
      );
    case 'note':
      return (
        <svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 24 24">
          <path d="m19 2v-2h-2v2h-2v-2h-2v2h-2v-2h-2v2h-2v-2h-2v2h-2v19a3 3 0 0 0 3 3h12a3 3 0 0 0 3-3v-19zm0 19a1 1 0 0 1 -1 1h-12a1 1 0 0 1 -1-1v-17h14zm-2-12h-10v-2h10zm0 4h-10v-2h10zm-4 4h-6v-2h6z"/>
        </svg>
      );
    default:
      return (<></>);
  }
}

export function Callout({ title, type, children }) {
  return (
    <div className={`callout ${type}`}>
      <span className={`type-name ${type}`}>
        {icon(type)} {type}
      </span>
      <strong>{title}</strong>
      <span>{children}</span>
      <style jsx>
        {`
          .type-name :global(svg) {
            width: 20px;
            height: 20px;
            margin-right: .4rem;
            vertical-align: bottom;
          }
          .type-name.warning :global(svg) {
            color: #990000;
            fill: #990000;
          }
          .type-name {
            position: absolute;
            top: -.9rem;
            left: 1rem;
            background-color: inherit;
            padding: 0 .5rem;
            text-transform: capitalize;
          }
          .callout {
            display: flex;
            flex-direction: column;
            padding: 1.2rem 1.5rem;
            background: #fff;
            border: 1px solid #dce6e9;
            border-radius: 4px;
            margin: 2rem 0;
            position: relative;
          }
          .callout :global(p) {
            margin: 0;
            font-size: 0.95rem;
            light-height: 1.75;
          }
          .callout.warning {
            color: #990000;
            border: 1px solid #aa0000;
            box-shadow: 0 1px 3px rgba(0, 0, 0, 0.1);
          }
        `}
      </style>
    </div>
  );
}
