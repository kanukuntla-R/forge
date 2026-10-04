// bundle-page.js — bundles a Next.js App Router page into a self-contained
// IIFE that awaits the page's default export and mounts its JSX result.
//
// Usage: node bundle-page.js <page.tsx> <props.json> <output.js> <tsconfig.json> [aliases.json]
//
// props.json: { params, searchParams } — passed to Page(props).
// aliases.json (optional): [{ specifier, kind: "prisma"|"drizzle", tableVars? }]
//   — data-access imports to replace with a generic fixture stub. Built by
//   Go (internal/renderer/pages.go's detectPageAliases); the stub SOURCE
//   itself is generated here in Node, per M13.2.5's Node-owns-fixture-data
//   decision — Go only decides *which* imports need stubbing.
const esbuild = require('esbuild');
const path = require('path');
const fs = require('fs');

const [pagePath, propsPath, outputPath, tsconfigPath, aliasesPath] = process.argv.slice(2);

const props = JSON.parse(fs.readFileSync(propsPath, 'utf-8'));
const aliases = aliasesPath ? JSON.parse(fs.readFileSync(aliasesPath, 'utf-8')) : [];

const entrySource = `
import React from 'react';
import { createRoot } from 'react-dom/client';
import Page from '${pagePath.replace(/\\/g, '/')}';

const props = ${JSON.stringify(props)};

(async () => {
    const el = await Page(props);
    const container = document.getElementById('root');
    createRoot(container).render(el);
})();
`;

// Generic, non-schema-aware placeholder rows: a small fixed set of
// commonly-useful fields, synthesized by type pattern (like GenerateProps
// does for component props) rather than a real project schema — Level A
// ships without per-table field detection (M13.2.5 decision 2).
function genericRowsSource() {
    return `
function genericRow(i) {
    return {
        id: i,
        name: 'Sample Name ' + i,
        email: 'user' + i + '@example.com',
        title: 'Sample Title ' + i,
        content: 'Sample content text.',
        slug: 'sample-slug-' + i,
        createdAt: new Date(0).toISOString(),
        published: true,
        approved: true,
    };
}
const FIXTURE_ROWS = [1, 2, 3].map(genericRow);
`;
}

// Proxy-based Prisma stub: any model property (prisma.user, prisma.post,
// ...) resolves to the same generic method surface, scoped to the
// operations forge's Prisma detector actually recognizes
// (internal/analyzer/prisma.go's prismaReadMethods/prismaWriteMethods).
function prismaStubSource() {
    return `
${genericRowsSource()}
function makeModelStub() {
    const rows = FIXTURE_ROWS;
    return {
        findMany: async () => rows,
        findUnique: async (args) => rows.find(r => r.id === args?.where?.id) ?? rows[0] ?? null,
        findUniqueOrThrow: async (args) => rows.find(r => r.id === args?.where?.id) ?? rows[0],
        findFirst: async () => rows[0] ?? null,
        findFirstOrThrow: async () => rows[0],
        count: async () => rows.length,
        aggregate: async () => ({}),
        groupBy: async () => [],
        create: async (args) => ({ id: rows.length + 1, ...genericRow(rows.length + 1), ...(args?.data || {}) }),
        createMany: async () => ({ count: 0 }),
        createManyAndReturn: async () => [],
        update: async (args) => ({ ...rows[0], ...(args?.data || {}) }),
        updateMany: async () => ({ count: 0 }),
        upsert: async (args) => ({ ...rows[0], ...(args?.create || {}) }),
        delete: async () => rows[0] ?? null,
        deleteMany: async () => ({ count: 0 }),
    };
}
export const prisma = new Proxy({}, { get: () => makeModelStub() });
`;
}

// Drizzle stub: covers exactly the three query shapes
// internal/analyzer/drizzle.go's matchDrizzleChain recognizes —
// select().from(), insert/update/delete(), and query.<table>.findMany/First.
// tableVars also get inert plain-object exports so `import { db, comments }
// from "@/db"` resolves and `eq(comments.postId, x)` doesn't throw (real
// drizzle-orm's eq() only duck-types its column arg; a plain object is safe).
function drizzleStubSource(tableVars) {
    // Skip anything that isn't a valid identifier (e.g. empty, from a stale
    // analysis.json) so one malformed entry can't break the whole bundle.
    const tableExports = tableVars
        .filter(v => typeof v === 'string' && /^[A-Za-z_$][\w$]*$/.test(v))
        .map(v => `export const ${v} = {};`).join('\n');
    return `
${genericRowsSource()}
// Drizzle's real query builder is "thenable" (awaitable directly, e.g.
// \`await db.select().from(x)\`) AND supports further chaining (e.g.
// \`await db.select().from(x).where(y)\`). This stub mirrors both: calling
// .where() re-returns a query result rather than the bare rows array, so
// the chain never hits "rows.where is not a function".
function queryResult() {
    return {
        where: () => queryResult(),
        then: (resolve) => resolve(FIXTURE_ROWS),
    };
}
export const db = {
    select: () => ({ from: () => queryResult() }),
    insert: () => ({ values: async () => [] }),
    update: () => ({ set: () => ({ where: async () => [] }) }),
    delete: () => ({ where: async () => [] }),
    query: new Proxy({}, { get: () => ({
        findMany: async () => FIXTURE_ROWS,
        findFirst: async () => FIXTURE_ROWS[0] ?? null,
    })}),
};
${tableExports}
`;
}

function stubSourceFor(alias) {
    return alias.kind === 'drizzle' ? drizzleStubSource(alias.tableVars || []) : prismaStubSource();
}

const aliasBySpecifier = new Map(aliases.map(a => [a.specifier, a]));

const dataAccessStubPlugin = {
    name: 'forge-data-access-stub',
    setup(build) {
        build.onResolve({ filter: /.*/ }, (args) => {
            if (aliasBySpecifier.has(args.path)) {
                return { path: args.path, namespace: 'forge-stub' };
            }
            return null;
        });
        build.onLoad({ filter: /.*/, namespace: 'forge-stub' }, (args) => {
            return { contents: stubSourceFor(aliasBySpecifier.get(args.path)), loader: 'js' };
        });
    },
};

esbuild.build({
    stdin: {
        contents: entrySource,
        resolveDir: path.dirname(pagePath),
        sourcefile: 'entry.jsx',
        loader: 'jsx',
    },
    bundle: true,
    format: 'iife',
    outfile: outputPath,
    platform: 'browser',
    // See bundle.js: esbuild's classic JSX transform needs each file to
    // import React itself; Next.js pages rely on the automatic runtime and
    // never do that, so without this every page silently rendered blank.
    jsx: 'automatic',
    tsconfig: tsconfigPath,
    // Next's internals reference process.env at module-load time; a banner
    // (not a same-file shim) is required because ESM imports execute before
    // any of the importing module's own top-level statements. Per the
    // M13.0.5 spike (docs/spikes/2026-08-08-m13-page-rendering-feasibility.md).
    banner: { js: "globalThis.process = globalThis.process || { env: { NODE_ENV: 'development' } };" },
    nodePaths: [path.join(__dirname, 'node_modules')],
    plugins: aliases.length > 0 ? [dataAccessStubPlugin] : [],
}).catch(err => {
    console.error(err.message || String(err));
    process.exit(1);
});
