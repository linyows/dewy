"use client";

import React from "react";
import { DocSearch } from "@docsearch/react";
import "@docsearch/css";

const Search: React.FC = () => {
  return (
    <DocSearch
      appId={process.env.NEXT_PUBLIC_ALGOLIA_APP_ID!}
      apiKey={process.env.NEXT_PUBLIC_ALGOLIA_API_KEY!}
      indices={["Doc website"]}
    />
  );
};

export default Search;