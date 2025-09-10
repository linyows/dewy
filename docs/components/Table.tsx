import * as React from 'react';

//export function Table({ children }) {
//  console.log(children);
//  return (
//    <table>
//    </table>
//  );
//}
type Align = "left" | "center" | "right";
type Variant = "default" | "striped" | "bordered";

type TableProps = {
  children?: React.ReactNode;
  variant?: Variant;
  dense?: boolean;
  stickyHeader?: boolean;
  align?: Align;
  caption?: string;
};

export function Table({ children, variant = "default", dense = false, stickyHeader = false, align = "left", caption }: TableProps) {
  const classnames = [
    variant !== "default" && `${variant}`,
    dense && "dense",
    stickyHeader && "sticky",
    align && `align-${align}`
  ].filter(Boolean).join(" ")
  return (
    <div className="table">
      <table className={classnames} >
        {caption ? <caption>{caption}</caption> : null}
        {children}
      </table>
      <style jsx>
        {`
          .table {
            display: block;
            padding: 1rem 0;
            max-width: 100%;
            min-width: var(--min-width);
            overflow-x: auto;
            -webkit-overflow-scrolling: touch;
          }
          .table table {
            width: 100%;
            border-collapse: collapse;
            table-layout: fixed;
            overflow: auto;
          }
          .table caption {
            caption-side: top;
            text-align: left;
            font-weight: 600;
            margin-bottom: .5rem;
          }
          .table :global(tr) {
            vertical-align: top;
          }
          .table :global(th) {
            font-weight: 600;
            background: var(--secondary-color);
          }
          .table :global(th),
          .table :global(td) {
            padding: .5rem 1rem;
            border: 1px solid var(--text-color);
            vertical-align: top;
            word-break: break-word;
            overflow-wrap: anywhere; 
          }
          .striped tbody tr:nth-child(odd) {
            background: var(--primary-color);
          }
          .bordered th,
          .bordered td {
            border-width: 2px;
          }
          .dense th,
          .dense td {
            padding: .5rem .75rem;
          }
          .align-left th,
          .align-left td {
            text-align: left;
          }
          .align-center th,
          .align-center td {
            text-align: center;
          }
          .align-right th,
          .align-right td {
            text-align: right;
          }
          .sticky thead th {
            position: sticky;
            top: 0;
            background: white;
            z-index: 1;
          }
          th[align="left"],
          td[align="left"] {
            text-align: left;
          }
          th[align="center"],
          td[align="center"]{
            text-align: center;
          }
          th[align="right"],
          td[align="right"] {
            text-align: right;
          }
        `}
      </style>
    </div>
  );
}