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
    // esbuild defaults to the classic JSX transform (React.createElement),
    // which requires each source file to import React itself. Modern
    // React/Next.js components rely on the automatic runtime and never do
    // that import, so classic mode silently threw "React is not defined"
    // inside every component, crashing the render but still writing a
    // valid, blank PNG (caught by internal/renderer/pipeline_test.go's
    // assertNotBlank, not by the earlier magic-bytes-only check).
    jsx: 'automatic',
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
