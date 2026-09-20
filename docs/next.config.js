const withMarkdoc = require('@markdoc/next.js');

module.exports = withMarkdoc({ mode: 'static' })({
  pageExtensions: ['js', 'jsx', 'ts', 'tsx', 'md', 'mdoc'],
  output: 'export',
  trailingSlash: false,
  images: {
    unoptimized: true,
  },
  // @algolia/autocomplete-core ships a UMD main without an exports map, so its
  // named exports are unresolvable when Node loads @docsearch/react as ESM.
  transpilePackages: ['@docsearch/react', '@algolia/autocomplete-core'],
});
