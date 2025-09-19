const withMarkdoc = require('@markdoc/next.js');

module.exports = withMarkdoc({ mode: 'static' })({
  pageExtensions: ['js', 'jsx', 'ts', 'tsx', 'md', 'mdoc'],
  output: 'export',
  trailingSlash: true,
  images: {
    unoptimized: true,
  },
});
