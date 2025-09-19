import * as React from 'react';

const icon = (name: string) => {
  switch (name) {
    case 'note':
      return (
        //<svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 24 24">
        //  <path d="m19 2v-2h-2v2h-2v-2h-2v2h-2v-2h-2v2h-2v-2h-2v2h-2v19a3 3 0 0 0 3 3h12a3 3 0 0 0 3-3v-19zm0 19a1 1 0 0 1 -1 1h-12a1 1 0 0 1 -1-1v-17h14zm-2-12h-10v-2h10zm0 4h-10v-2h10zm-4 4h-6v-2h6z"/>
        //</svg>
        <svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 24 24">
          <path d="M12,0A12,12,0,1,0,24,12,12.013,12.013,0,0,0,12,0Zm0,22A10,10,0,1,1,22,12,10.011,10.011,0,0,1,12,22Z"/>
          <path d="M12,10H11a1,1,0,0,0,0,2h1v6a1,1,0,0,0,2,0V12A2,2,0,0,0,12,10Z"/>
          <circle cx="12" cy="6.5" r="1.5"/>
        </svg>
      );
    case 'tip':
      return (
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" width="512" height="512">
          <path d="m8 20.149v3.851h8v-3.685a6.005 6.005 0 0 1 1.932-4.552 9 9 0 1 0 -12.064-.18 6.263 6.263 0 0 1 2.132 4.566zm6 1.851h-4v-1.851c0-.05-.007-.1-.008-.149h4.024c0 .105-.016.209-.016.315zm-8.94-13.925a7 7 0 1 1 11.553 6.184 7.655 7.655 0 0 0 -2.293 3.741h-1.32v-7.184a3 3 0 0 0 2-2.816h-2a1 1 0 0 1 -2 0h-2a3 3 0 0 0 2 2.816v7.184h-1.322a8.634 8.634 0 0 0 -2.448-3.881 6.96 6.96 0 0 1 -2.17-6.044z"/>
        </svg>
      );
    case 'important':
      return (
        <svg xmlns="http://www.w3.org/2000/svg" width="512" height="512" viewBox="0 0 24 24">
          <path d="M20.137,24a2.8,2.8,0,0,1-1.987-.835L12,17.051,5.85,23.169a2.8,2.8,0,0,1-3.095.609A2.8,2.8,0,0,1,1,21.154V5A5,5,0,0,1,6,0H18a5,5,0,0,1,5,5V21.154a2.8,2.8,0,0,1-1.751,2.624A2.867,2.867,0,0,1,20.137,24ZM6,2A3,3,0,0,0,3,5V21.154a.843.843,0,0,0,1.437.6h0L11.3,14.933a1,1,0,0,1,1.41,0l6.855,6.819a.843.843,0,0,0,1.437-.6V5a3,3,0,0,0-3-3Z"/>
        </svg>
      )
    case 'warning':
      return (
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
          <path d="m23.121,6.151L17.849.878c-.567-.566-1.321-.878-2.121-.878h-7.455c-.8,0-1.554.312-2.122.879L.879,6.151c-.566.567-.879,1.32-.879,2.121v7.456c0,.801.312,1.554.879,2.121l5.272,5.273c.567.566,1.321.878,2.121.878h7.455c.8,0,1.554-.312,2.122-.879l5.271-5.272c.566-.567.879-1.32.879-2.121v-7.456c0-.801-.313-1.554-.879-2.121Zm-1.121,9.577c0,.263-.106.521-.293.707l-5.271,5.271c-.19.189-.442.294-.709.294h-7.455c-.267,0-.519-.104-.708-.293l-5.271-5.272c-.187-.187-.293-.444-.293-.707v-7.456c0-.263.106-.521.293-.707L7.563,2.294c.19-.189.442-.294.709-.294h7.455c.267,0,.519.104.708.293l5.271,5.272c.187.187.293.444.293.707v7.456Zm-9-2.728h-2v-7h2v7Zm.5,3.5c0,.828-.672,1.5-1.5,1.5s-1.5-.672-1.5-1.5.672-1.5,1.5-1.5,1.5.672,1.5,1.5Z"/>
        </svg>
      );
    case 'caution':
      return (
        <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24">
          <path d="m23.121,6.151L17.849.878c-.567-.566-1.321-.878-2.121-.878h-7.455c-.8,0-1.554.312-2.122.879L.879,6.151c-.566.567-.879,1.32-.879,2.121v7.456c0,.801.312,1.554.879,2.121l5.272,5.273c.567.566,1.321.878,2.121.878h7.455c.8,0,1.554-.312,2.122-.879l5.271-5.272c.566-.567.879-1.32.879-2.121v-7.456c0-.801-.313-1.554-.879-2.121Zm-1.121,9.577c0,.263-.106.521-.293.707l-5.271,5.271c-.19.189-.442.294-.709.294h-7.455c-.267,0-.519-.104-.708-.293l-5.271-5.272c-.187-.187-.293-.444-.293-.707v-7.456c0-.263.106-.521.293-.707L7.563,2.294c.19-.189.442-.294.709-.294h7.455c.267,0,.519.104.708.293l5.271,5.272c.187.187.293.444.293.707v7.456Zm-9-2.728h-2v-7h2v7Zm.5,3.5c0,.828-.672,1.5-1.5,1.5s-1.5-.672-1.5-1.5.672-1.5,1.5-1.5,1.5.672,1.5,1.5Z"/>
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
          .type-name.note :global(svg) {
            color: var(--note-color);
            fill: var(--note-color);
          }
          .type-name.tip :global(svg) {
            color: var(--tip-color);
            fill: var(--tip-color);
          }
          .type-name.important :global(svg) {
            color: var(--important-color);
            fill: var(--important-color);
          }
          .type-name.warning :global(svg) {
            color: var(--warning-color);
            fill: var(--warning-color);
          }
          .type-name.caution :global(svg) {
            color: var(--caution-color);
            fill: var(--caution-color);
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
            padding: 1.2rem 1.5rem 1.2rem 3.4rem;
            background: #fff;
            border: 1px solid #dce6e9;
            border-radius: 4px;
            margin: 2rem 0;
            position: relative;
            box-shadow: 5px 5px 1px rgba(0, 0, 0, 0.1);
          }
          .callout :global(p) {
            margin: 0;
            font-size: 0.95rem;
            line-height: 1.75;
          }
          .callout.note {
            color: var(--note-color);
            border-color: var(--note-color);
          }
          .callout.tip {
            color: var(--tip-color);
            border-color: var(--tip-color);
          }
          .callout.important {
            color: var(--important-color);
            border-color: var(--important-color);
          }
          .callout.warning {
            color: var(--warning-color);
            border-color: var(--warning-color);
          }
          .callout.caution {
            color: var(--caution-color);
            border-color: var(--caution-color);
          }
        `}
      </style>
    </div>
  );
}
