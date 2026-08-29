// bundle.js — bundles a component + synthetic props into a self-contained IIFE.
//
// Usage: node bundle.js <component.tsx> <props.json> <output.js> [tsconfig.json]
//
// The optional tsconfig.json path lets esbuild resolve the project's path
// aliases (e.g. "@/components/Foo") the same way the TypeScript compiler
// would. Omit it to bundle without alias resolution (M13.1 behavior).
const esbuild = require('esbuild');
const path = require('path');
const fs = require('fs');

const [componentPath, propsPath, outputPath, tsconfigPath] = process.argv.slice(2);

const props = JSON.parse(fs.readFileSync(propsPath, 'utf-8'));

const entrySource = `
import React from 'react';
import { createRoot } from 'react-dom/client';
import Component from '${componentPath.replace(/\\/g, '/')}';

const props = ${JSON.stringify(props)};
const container = document.getElementById('root');
const root = createRoot(container);
root.render(React.createElement(Component, props));
`;

const buildOptions = {
    stdin: {
        contents: entrySource,
        resolveDir: path.dirname(componentPath),
        sourcefile: 'entry.jsx',
        loader: 'jsx',
    },
    bundle: true,
    format: 'iife',
    outfile: outputPath,
    platform: 'browser',
    // react/react-dom live in this script's own node_modules (the persistent
    // renderer cache dir), not next to the component being bundled.
    nodePaths: [path.join(__dirname, 'node_modules')],
};
if (tsconfigPath) {
    buildOptions.tsconfig = tsconfigPath;
}

esbuild.build(buildOptions).catch(err => {
    console.error(err.message || String(err));
    process.exit(1);
});
