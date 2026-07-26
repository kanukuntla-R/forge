const { useState, useEffect, useRef } = React;

const THEME_STORAGE_KEY = 'forge-theme';
const THEME_MODES = ['light', 'dark', 'auto'];

// ── Helpers ──────────────────────────────────────────────────────────────────

function buildTree(files) {
    const root = { name: '', path: '', files: [], dirs: {} };
    for (const f of files) {
        const parts = f.path.split('/');
        const fileName = parts[parts.length - 1];
        const dirParts = parts.slice(0, -1);
        let node = root;
        let currentPath = '';
        for (const part of dirParts) {
            currentPath = currentPath ? currentPath + '/' + part : part;
            if (!node.dirs[part]) {
                node.dirs[part] = { name: part, path: currentPath, files: [], dirs: {} };
            }
            node = node.dirs[part];
        }
        node.files.push({ name: fileName, path: f.path, fileInfo: f });
    }
    return root;
}

function countTotalFiles(dirNode) {
    let count = dirNode.files.length;
    for (const sub of Object.values(dirNode.dirs)) count += countTotalFiles(sub);
    return count;
}

function sortedDirs(dirNode) {
    return Object.values(dirNode.dirs).sort((a, b) => a.name.localeCompare(b.name));
}

function sortedFiles(dirNode) {
    return [...dirNode.files].sort((a, b) => a.name.localeCompare(b.name));
}

// Look up the original target URL for an api_call that has no matched route.
// api_calls only carries to_route (matched URL) — the raw fetch target lives in
// FileInfo.calls[].target, correlated by from_file + line.
function getCallTarget(apiCall, files) {
    const file = files.find(f => f.path === apiCall.from_file);
    if (!file || !file.calls) return '';
    const call = file.calls.find(c => c.line === apiCall.line);
    return call ? call.target : '';
}

function buildGraphData(analysis) {
    const nextjs    = analysis.frameworks && analysis.frameworks.nextjs;
    const fastapi   = analysis.frameworks && analysis.frameworks.fastapi;
    const databases = analysis.databases || [];

    const nodes   = [];
    const edges   = [];
    const nodeIds = new Set();

    // Nodes for both frameworks must exist before either framework's edges
    // are built — a cross-framework api_call edge (e.g. a Next.js page
    // calling a FastAPI route) needs the OTHER framework's node to already
    // be in nodeIds, regardless of which framework's edges are built first.
    // Same reasoning extends to database tables: a query edge needs its
    // table node to exist, so table nodes are added alongside framework nodes.
    if (nextjs) addNextJSNodes(nextjs, nodes, nodeIds);
    if (fastapi) addFastAPINodes(fastapi, nodes, nodeIds);
    if (databases.length > 0) addDatabaseNodes(databases, nodes, nodeIds);
    if (nextjs) addNextJSEdges(nextjs, analysis, edges, nodeIds);
    if (fastapi) addFastAPIEdges(fastapi, analysis, edges, nodeIds);
    if (databases.length > 0) addDatabaseEdges(analysis, edges, nodeIds);

    return { nodes, edges };
}

function addDatabaseNodes(databases, nodes, nodeIds) {
    for (const db of databases) {
        for (const table of (db.tables || [])) {
            const id = 'db-table:' + db.type + ':' + table.name;
            if (!nodeIds.has(id)) {
                nodes.push({
                    data: { id, label: table.name, type: 'table', file: table.file, orm: db.type, framework: 'database' }
                });
                nodeIds.add(id);
            }
        }
    }
}

// findQuerySourceNode resolves the file that issued a query to its graph
// node id. Next.js pages/routes/components use the bare file path as their
// node id; FastAPI uses prefixed ids (see addFastAPINodes). If the file
// doesn't own any graph node (e.g. a plain db-helper module with no
// page/route/component of its own), the query has no edge — dropped rather
// than inventing a generic "file" node type. Revisit if that gap shows up
// in practice (v0.5.1).
function findQuerySourceNode(filePath, analysis, nodeIds) {
    if (nodeIds.has(filePath)) return filePath;

    const fastapi = analysis.frameworks && analysis.frameworks.fastapi;
    if (fastapi) {
        const route = (fastapi.routes || []).find(r => r.file === filePath);
        if (route) {
            const id = 'fastapi-route:' + route.method + ':' + route.path;
            if (nodeIds.has(id)) return id;
        }
        const app = (fastapi.apps || []).find(a => a.file === filePath);
        if (app) {
            const id = 'fastapi-app:' + app.file;
            if (nodeIds.has(id)) return id;
        }
    }
    return null;
}

function addDatabaseEdges(analysis, edges, nodeIds) {
    let edgeIdx = 0;
    for (const file of (analysis.files || [])) {
        for (const query of (file.database_queries || [])) {
            const targetId = 'db-table:' + query.orm + ':' + query.table;
            if (!nodeIds.has(targetId)) continue;
            const sourceId = findQuerySourceNode(file.path, analysis, nodeIds);
            if (!sourceId) continue;
            edges.push({
                data: {
                    id: 'query-' + (edgeIdx++),
                    source: sourceId,
                    target: targetId,
                    type: 'query',
                    operation: query.operation.toUpperCase(),
                    confidence: query.confidence,
                    orm: query.orm,
                }
            });
        }
    }
}

function addNextJSNodes(nextjs, nodes, nodeIds) {
    for (const page of (nextjs.pages || [])) {
        const id = page.file;
        if (!nodeIds.has(id)) {
            nodes.push({ data: { id, label: page.path, type: 'page', file: page.file } });
            nodeIds.add(id);
        }
    }

    for (const route of (nextjs.routes || [])) {
        const id = route.file;
        if (!nodeIds.has(id)) {
            nodes.push({ data: { id, label: route.path, type: 'route', file: route.file, methods: route.methods } });
            nodeIds.add(id);
        }
    }

    for (const comp of (nextjs.components || [])) {
        const id = comp.file;
        if (!nodeIds.has(id)) {
            nodes.push({ data: { id, label: comp.name, type: 'component', file: comp.file } });
            nodeIds.add(id);
        }
    }
}

function addNextJSEdges(nextjs, analysis, edges, nodeIds) {
    // API call edges — only emit if both endpoints are graph nodes.
    // to_framework names the framework that OWNS the matched route (empty
    // means it's one of this project's own Next.js routes); the fallback
    // framework's routes/node-id scheme differ, so the lookup branches on it.
    for (let i = 0; i < (nextjs.api_calls || []).length; i++) {
        const call = nextjs.api_calls[i];
        if (!call.to_route) continue;
        let targetId;
        if (call.to_framework === 'fastapi') {
            const fastapi = analysis.frameworks && analysis.frameworks.fastapi;
            const targetRoute = ((fastapi && fastapi.routes) || []).find(r => r.path === call.to_route);
            if (!targetRoute) continue;
            targetId = 'fastapi-route:' + targetRoute.method + ':' + targetRoute.path;
        } else {
            const targetRoute = (nextjs.routes || []).find(r => r.path === call.to_route);
            if (!targetRoute) continue;
            targetId = targetRoute.file;
        }
        if (!nodeIds.has(call.from_file) || !nodeIds.has(targetId)) continue;
        edges.push({
            data: {
                id: 'api-' + i,
                source: call.from_file,
                target: targetId,
                type: 'api_call',
                method: call.method || 'GET',
                confidence: call.confidence,
            }
        });
    }

    // Import edges — only between known graph nodes
    let importIdx = 0;
    for (const file of (analysis.files || [])) {
        if (!file.imports) continue;
        for (const imp of file.imports) {
            if (!imp.resolved || imp.external) continue;
            if (!nodeIds.has(file.path) || !nodeIds.has(imp.resolved)) continue;
            edges.push({
                data: {
                    id: 'import-' + (importIdx++),
                    source: file.path,
                    target: imp.resolved,
                    type: 'import',
                }
            });
        }
    }

    // Component usage edges
    let usageIdx = 0;
    for (const comp of (nextjs.components || [])) {
        for (const user of (comp.used_by || [])) {
            if (!nodeIds.has(user) || !nodeIds.has(comp.file)) continue;
            edges.push({
                data: {
                    id: 'usage-' + (usageIdx++),
                    source: user,
                    target: comp.file,
                    type: 'usage',
                }
            });
        }
    }

    // Dedup: if both an import and a usage edge exist between the same pair,
    // keep only the usage edge (more meaningful for component relationships).
    const usagePairs = new Set(
        edges.filter(e => e.data.type === 'usage')
             .map(e => e.data.source + '|' + e.data.target)
    );
    for (let i = edges.length - 1; i >= 0; i--) {
        const e = edges[i];
        if (e.data.type === 'import' && usagePairs.has(e.data.source + '|' + e.data.target)) {
            edges.splice(i, 1);
        }
    }
}

function addFastAPINodes(fastapi, nodes, nodeIds) {
    for (const app of (fastapi.apps || [])) {
        const id = 'fastapi-app:' + app.file;
        if (!nodeIds.has(id)) {
            nodes.push({ data: { id, label: app.name, type: 'page', file: app.file, framework: 'fastapi' } });
            nodeIds.add(id);
        }
    }

    for (const router of (fastapi.routers || [])) {
        const id = 'fastapi-router:' + router.file + ':' + router.name;
        if (!nodeIds.has(id)) {
            nodes.push({ data: { id, label: router.prefix || router.name, type: 'router', file: router.file, framework: 'fastapi' } });
            nodeIds.add(id);
        }
    }

    for (const route of (fastapi.routes || [])) {
        const id = 'fastapi-route:' + route.method + ':' + route.path;
        if (!nodeIds.has(id)) {
            nodes.push({
                data: {
                    id, type: 'route', file: route.file, framework: 'fastapi',
                    label: route.method + ' ' + route.path + '\n' + route.handler,
                }
            });
            nodeIds.add(id);
        }
    }
}

function addFastAPIEdges(fastapi, analysis, edges, nodeIds) {
    // Router → app edges (include_router), only for reachable routers.
    let inclusionIdx = 0;
    for (const router of (fastapi.routers || [])) {
        if (!router.reachable) continue;
        const routerId = 'fastapi-router:' + router.file + ':' + router.name;
        const app = (fastapi.apps || [])[0];
        if (!app) continue;
        const appId = 'fastapi-app:' + app.file;
        if (!nodeIds.has(routerId) || !nodeIds.has(appId)) continue;
        edges.push({ data: { id: 'router-inclusion-' + (inclusionIdx++), source: routerId, target: appId, type: 'router_inclusion' } });
    }

    // Route → router (or app if no router) edges, same visual type as inclusion.
    for (const route of (fastapi.routes || [])) {
        const routeId = 'fastapi-route:' + route.method + ':' + route.path;
        let parentId = null;
        if (route.router_name) {
            const router = (fastapi.routers || []).find(r => r.name === route.router_name && r.file === route.file);
            if (router) parentId = 'fastapi-router:' + router.file + ':' + router.name;
        }
        if (!parentId) {
            const app = (fastapi.apps || [])[0];
            if (app) parentId = 'fastapi-app:' + app.file;
        }
        if (!parentId || !nodeIds.has(routeId) || !nodeIds.has(parentId)) continue;
        edges.push({ data: { id: 'router-inclusion-' + (inclusionIdx++), source: routeId, target: parentId, type: 'router_inclusion' } });
    }

    // api_calls edges — only emit if both endpoints are already graph nodes
    // (caller file may be a plain Python script with no FastAPI node of its own).
    // to_framework names the framework that OWNS the matched route (empty
    // means it's one of this project's own FastAPI routes).
    for (let i = 0; i < (fastapi.api_calls || []).length; i++) {
        const call = fastapi.api_calls[i];
        if (!call.to_route) continue;
        let targetId;
        if (call.to_framework === 'nextjs') {
            const nextjs = analysis && analysis.frameworks && analysis.frameworks.nextjs;
            const targetRoute = ((nextjs && nextjs.routes) || []).find(r => r.path === call.to_route);
            if (!targetRoute) continue;
            targetId = targetRoute.file;
        } else {
            const targetRoute = (fastapi.routes || []).find(r => r.path === call.to_route);
            if (!targetRoute) continue;
            targetId = 'fastapi-route:' + targetRoute.method + ':' + targetRoute.path;
        }
        if (!nodeIds.has(call.file) || !nodeIds.has(targetId)) continue;
        edges.push({
            data: {
                id: 'fastapi-api-' + i,
                source: call.file,
                target: targetId,
                type: 'api_call',
                method: call.method || 'GET',
                confidence: call.confidence,
            }
        });
    }
}

// ── Constants ─────────────────────────────────────────────────────────────────

const CONFIDENCE_CLASS = {
    high:   'bg-green-100 text-green-700',
    medium: 'bg-yellow-100 text-yellow-700',
    low:    'bg-red-100 text-red-700',
};

const METHOD_CLASS = {
    GET:     'bg-blue-100 text-blue-700',
    POST:    'bg-green-100 text-green-700',
    PUT:     'bg-yellow-100 text-yellow-700',
    PATCH:   'bg-yellow-100 text-yellow-700',
    DELETE:  'bg-red-100 text-red-700',
};

// cose layout options, shared by every place that runs the graph layout so
// spacing tuning can't drift out of sync between call sites.
const coseLayoutOptions = {
    name: 'cose-bilkent',
    padding: 60,
    nodeRepulsion: 8000,
    idealEdgeLength: 100,
    numIter: 2500,
    nodeOverlap: 30,
    randomize: false,
    fit: true,
    tile: true,
    tilingPaddingVertical: 15,
    tilingPaddingHorizontal: 15,
    quality: 'default',
};

// Reads current CSS custom property values off :root / [data-theme="dark"].
// Called at cytoscape init and whenever the theme changes, so styles always
// reflect whichever theme is active at that moment.
function getThemeColors() {
    const s = getComputedStyle(document.documentElement);
    const v = name => s.getPropertyValue(name).trim();
    return {
        bg: v('--bg'),
        panel: v('--panel'),
        nodePage: v('--node-page'), nodeRoute: v('--node-route'),
        nodeRouter: v('--node-router'), nodeComponent: v('--node-component'),
        nodeTable: v('--node-table'), nodeTableBorder: v('--node-table-border'),
        edgeApi: v('--edge-api'), edgeImport: v('--edge-import'),
        edgeUsage: v('--edge-usage'), edgeRouter: v('--edge-router'),
        edgeQuery: v('--edge-query'),
    };
}

// Cytoscape doesn't read CSS custom properties itself — its style array needs
// resolved color values, so this rebuilds it from getThemeColors() output.
function buildCytoscapeStyle(c) {
    return [
    {
        selector: 'node[type="page"]',
        style: {
            'shape':        'round-rectangle',
            'background-color': c.nodePage,
            'label':        'data(label)',
            'color':        '#fff',
            'text-valign':  'center',
            'text-halign':  'center',
            'font-size':    '11px',
            'font-family':  'monospace',
            'width':        'label',
            'height':       28,
            'padding':      '8px',
            'border-width': 0,
        }
    },
    {
        selector: 'node[type="route"]',
        style: {
            'shape':        'hexagon',
            'background-color': c.nodeRoute,
            'label':        'data(label)',
            'text-wrap':    'wrap',
            'color':        '#fff',
            'text-valign':  'center',
            'text-halign':  'center',
            'font-size':    '10px',
            'font-family':  'monospace',
            'width':        'label',
            'height':       'label',
            'padding':      '10px',
            'border-width': 0,
        }
    },
    {
        selector: 'node[type="router"]',
        style: {
            'shape':        'hexagon',
            'background-color': c.nodeRouter,
            'label':        'data(label)',
            'text-wrap':    'wrap',
            'color':        '#fff',
            'text-valign':  'center',
            'text-halign':  'center',
            'font-size':    '11px',
            'font-family':  'monospace',
            'width':        'label',
            'height':       32,
            'padding':      '8px',
            'border-width': 0,
        }
    },
    {
        selector: 'node[type="component"]',
        style: {
            'shape':        'ellipse',
            'background-color': c.nodeComponent,
            'label':        'data(label)',
            'color':        '#fff',
            'text-valign':  'center',
            'text-halign':  'center',
            'font-size':    '10px',
            'font-family':  'monospace',
            'width':        'label',
            'height':       'label',
            'padding':      '10px',
            'border-width': 0,
        }
    },
    {
        selector: 'node[type="table"]',
        style: {
            'shape':        'ellipse',
            'background-color': c.nodeTable,
            'label':        'data(label)',
            'text-valign':  'center',
            'text-halign':  'center',
            'color':        '#1c1917',
            'font-size':    '11px',
            'font-family':  'monospace',
            'font-weight':  'bold',
            'width':        60,
            'height':       40,
            'border-width': 2,
            'border-color': c.nodeTableBorder,
        }
    },
    {
        selector: 'edge[type="api_call"]',
        style: {
            'width':                  2,
            'line-color':             c.edgeApi,
            'target-arrow-color':     c.edgeApi,
            'target-arrow-shape':     'triangle',
            'curve-style':            'bezier',
            'label':                  'data(method)',
            'font-size':              '9px',
            'color':                  c.edgeApi,
            'text-background-color':  c.bg,
            'text-background-opacity': 1,
            'text-background-padding': '2px',
        }
    },
    {
        selector: 'edge[type="import"]',
        style: {
            'width':              1,
            'line-color':         c.edgeImport,
            'target-arrow-color': c.edgeImport,
            'target-arrow-shape': 'triangle',
            'curve-style':        'bezier',
            'line-style':         'dashed',
        }
    },
    {
        selector: 'edge[type="usage"]',
        style: {
            'width':              1.5,
            'line-color':         c.edgeUsage,
            'target-arrow-color': c.edgeUsage,
            'target-arrow-shape': 'triangle',
            'curve-style':        'bezier',
        }
    },
    {
        selector: 'edge[type="router_inclusion"]',
        style: {
            'width':              2,
            'line-color':         c.edgeRouter,
            'target-arrow-color': c.edgeRouter,
            'target-arrow-shape': 'triangle',
            'curve-style':        'bezier',
        }
    },
    {
        selector: 'edge[type="query"]',
        style: {
            'width':                   2,
            'line-color':              c.edgeQuery,
            'target-arrow-color':      c.edgeQuery,
            'target-arrow-shape':      'triangle',
            'curve-style':             'bezier',
            'label':                   'data(operation)',
            'font-size':               '9px',
            'color':                   c.edgeQuery,
            'text-background-color':   c.bg,
            'text-background-opacity': 1,
            'text-background-padding': '2px',
        }
    },
    {
        selector: '.faded',
        style: { 'opacity': 0.2 }
    },
    {
        selector: '.highlighted',
        style: {
            'border-width': 3,
            'border-color': '#fbbf24',
        }
    },
    ];
}

// ── Shared primitives ─────────────────────────────────────────────────────────

function MethodBadge({ method }) {
    const cls = METHOD_CLASS[method] || 'bg-[var(--panel-alt)] text-[var(--fg-dim)]';
    return React.createElement('span', {
        className: 'text-xs px-1.5 py-0.5 rounded font-mono font-medium ' + cls
    }, method);
}

function ReachableBadge({ reachable }) {
    const cls = reachable ? 'bg-green-100 text-green-700' : 'bg-[var(--panel-alt)] text-[var(--fg-muted)]';
    return React.createElement('span', {
        className: 'text-xs px-1.5 py-0.5 rounded ' + cls
    }, reachable ? 'reachable' : 'unreachable');
}

function ClickableFile({ path, onNavigate }) {
    return React.createElement('button', {
        className: 'font-mono text-xs text-blue-500 hover:text-blue-700 underline cursor-pointer',
        onClick: () => onNavigate(path),
    }, path);
}

function DetailSection({ title, count, children }) {
    return React.createElement('div', { className: 'mb-6' },
        React.createElement('h3', {
            className: 'text-xs font-semibold uppercase tracking-wide text-[var(--fg-muted)] mb-1.5'
        }, title + ' (' + count + ')'),
        children
    );
}

function makeToggle(label, color, active, onClick) {
    return React.createElement('button', {
        onClick,
        className: 'flex items-center gap-1.5 px-2 py-1 rounded text-xs cursor-pointer transition-colors ' +
            (active ? 'bg-[var(--panel-alt)]' : 'opacity-50 hover:opacity-75'),
    },
        React.createElement('span', {
            className: 'w-2 h-2 rounded-full flex-shrink-0',
            style: { backgroundColor: color }
        }),
        React.createElement('span', null, label)
    );
}

// ── Files view row components ─────────────────────────────────────────────────

function ImportRow({ imp, onNavigate }) {
    const names = imp.names && imp.names.length > 0 ? imp.names.join(', ') : '(side-effect)';
    return React.createElement('div', { className: 'flex items-start gap-1.5 py-0.5 text-sm' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] mt-0.5 select-none' }, '•'),
        React.createElement('div', { className: 'min-w-0' },
            React.createElement('span', { className: 'font-mono text-[var(--fg-dim)]' }, imp.source),
            React.createElement('span', { className: 'text-[var(--fg-muted)] mx-1.5' }, '·'),
            React.createElement('span', { className: 'text-[var(--fg-dim)]' }, names),
            imp.external
                ? React.createElement('span', {
                    className: 'ml-2 bg-[var(--panel-alt)] text-[var(--fg-muted)] text-xs px-1.5 py-0.5 rounded'
                  }, 'ext')
                : imp.resolved
                    ? React.createElement('span', { className: 'ml-2 text-[var(--fg-muted)]' },
                        '→ ',
                        React.createElement('button', {
                            className: 'font-mono text-blue-600 underline hover:text-blue-800 cursor-pointer',
                            onClick: () => onNavigate(imp.resolved),
                        }, imp.resolved))
                    : null
        )
    );
}

function ExportRow({ exp }) {
    return React.createElement('div', { className: 'flex items-center gap-2 py-0.5 text-sm' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-[var(--fg-dim)]' }, exp.name),
        React.createElement('span', { className: 'bg-[var(--panel-alt)] text-[var(--fg-muted)] text-xs px-1.5 py-0.5 rounded' }, exp.type),
        React.createElement('span', { className: 'text-[var(--fg-muted)] text-xs' }, 'line ' + exp.line)
    );
}

function DeclarationRow({ decl }) {
    return React.createElement('div', { className: 'flex items-center gap-2 py-0.5 text-sm' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-[var(--fg-dim)]' }, decl.name),
        React.createElement('span', { className: 'bg-[var(--panel-alt)] text-[var(--fg-muted)] text-xs px-1.5 py-0.5 rounded' }, decl.type),
        React.createElement('span', { className: 'text-[var(--fg-muted)] text-xs' }, 'line ' + decl.line)
    );
}

function CallRow({ call }) {
    const cls = CONFIDENCE_CLASS[call.confidence] || 'bg-[var(--panel-alt)] text-[var(--fg-dim)]';
    return React.createElement('div', { className: 'flex items-center gap-2 py-0.5 text-sm' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-[var(--fg-dim)]' }, call.target),
        call.method && React.createElement(MethodBadge, { method: call.method }),
        React.createElement('span', { className: 'text-xs px-1.5 py-0.5 rounded ' + cls }, call.confidence)
    );
}

function DatabaseQueryFileRow({ query }) {
    const cls = CONFIDENCE_CLASS[query.confidence] || 'bg-[var(--panel-alt)] text-[var(--fg-dim)]';
    return React.createElement('div', { className: 'flex items-center gap-2 py-0.5 text-sm' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-[var(--fg-dim)]' }, query.orm + '.' + query.table),
        React.createElement('span', { className: 'text-xs px-1.5 py-0.5 rounded bg-amber-50 text-amber-700 font-mono' }, query.operation.toUpperCase()),
        React.createElement('span', { className: 'text-xs px-1.5 py-0.5 rounded ' + cls }, query.confidence),
        React.createElement('span', { className: 'text-[var(--fg-muted)] text-xs' }, 'line ' + query.line)
    );
}

// ── FileDetails ───────────────────────────────────────────────────────────────

function FileDetails({ file, onNavigate, showDatabases }) {
    if (!file) {
        return React.createElement('div', {
            className: 'flex-1 flex items-center justify-center flex-col gap-1 text-sm'
        },
            React.createElement('span', { className: 'text-[var(--fg-muted)]' }, 'Select a file to view details'),
            React.createElement('span', { className: 'text-[var(--fg-muted)] text-xs' }, 'or use the Routes tab to see API connections')
        );
    }

    const imports        = file.imports          || [];
    const exports_       = file.exports          || [];
    const declarations   = file.declarations     || [];
    const calls          = file.calls            || [];
    const databaseQueries = file.database_queries || [];
    const none = React.createElement('p', { className: 'text-sm text-[var(--fg-muted)] italic' }, 'None');

    return React.createElement('div', { className: 'p-6 max-w-3xl' },
        React.createElement('h2', { className: 'font-mono font-semibold text-base mb-1 break-all' }, file.path),
        React.createElement('div', { className: 'flex gap-4 text-xs text-[var(--fg-muted)] mb-5 pb-4 border-b border-[var(--border)]' },
            React.createElement('span', null, file.language),
            React.createElement('span', null, file.size_bytes + ' bytes')
        ),
        React.createElement(DetailSection, { title: 'Imports', count: imports.length },
            imports.length > 0
                ? imports.map((imp, i) => React.createElement(ImportRow, { key: i, imp, onNavigate }))
                : none
        ),
        React.createElement(DetailSection, { title: 'Exports', count: exports_.length },
            exports_.length > 0
                ? exports_.map((exp, i) => React.createElement(ExportRow, { key: i, exp }))
                : none
        ),
        React.createElement(DetailSection, { title: 'Declarations', count: declarations.length },
            declarations.length > 0
                ? declarations.map((decl, i) => React.createElement(DeclarationRow, { key: i, decl }))
                : none
        ),
        React.createElement(DetailSection, { title: 'API Calls', count: calls.length },
            calls.length > 0
                ? calls.map((call, i) => React.createElement(CallRow, { key: i, call }))
                : none
        ),
        showDatabases && React.createElement(DetailSection, { title: 'Database Queries', count: databaseQueries.length },
            databaseQueries.length > 0
                ? databaseQueries.map((q, i) => React.createElement(DatabaseQueryFileRow, { key: i, query: q }))
                : none
        )
    );
}

// ── File tree components ──────────────────────────────────────────────────────

function FileNode({ file, depth, isSelected, onSelect }) {
    return React.createElement('div', {
        className: 'flex items-center py-0.5 cursor-pointer ' +
            (isSelected ? 'bg-blue-100' : 'hover:bg-[var(--panel-alt)]'),
        style: { paddingLeft: (depth * 16 + 20) + 'px', paddingRight: '8px' },
        onClick: onSelect,
    },
        React.createElement('span', {
            className: 'font-mono text-sm truncate ' +
                (isSelected ? 'text-blue-700 font-medium' : 'text-[var(--fg-dim)]')
        }, file.name)
    );
}

function DirectoryNode({ dir, depth, expandedDirs, onToggle, selectedFile, onSelectFile }) {
    const isExpanded = expandedDirs.has(dir.path);
    const count = countTotalFiles(dir);
    return React.createElement('div', null,
        React.createElement('div', {
            className: 'flex items-center gap-1 py-0.5 cursor-pointer select-none hover:bg-[var(--panel-alt)]',
            style: { paddingLeft: (depth * 16 + 4) + 'px', paddingRight: '8px' },
            onClick: () => onToggle(dir.path),
        },
            React.createElement('span', { className: 'text-[var(--fg-muted)] text-xs w-3 flex-shrink-0' },
                isExpanded ? '▾' : '▸'),
            React.createElement('span', { className: 'font-mono text-sm text-[var(--fg-dim)]' }, dir.name + '/'),
            React.createElement('span', { className: 'ml-1 text-xs text-[var(--fg-muted)]' }, '(' + count + ')')
        ),
        isExpanded && React.createElement('div', null,
            sortedDirs(dir).map(sub =>
                React.createElement(DirectoryNode, {
                    key: sub.path, dir: sub, depth: depth + 1,
                    expandedDirs, onToggle, selectedFile, onSelectFile,
                })
            ),
            sortedFiles(dir).map(f =>
                React.createElement(FileNode, {
                    key: f.path, file: f, depth: depth + 1,
                    isSelected: selectedFile === f.path,
                    onSelect: () => onSelectFile(f.path),
                })
            )
        )
    );
}

function FileTree({ tree, expandedDirs, onToggle, selectedFile, onSelectFile }) {
    if (tree.files.length === 0 && Object.keys(tree.dirs).length === 0) {
        return React.createElement('div', { className: 'p-4 text-sm text-[var(--fg-muted)] italic' },
            'No files in this project');
    }
    return React.createElement('div', { className: 'py-2' },
        sortedDirs(tree).map(dir =>
            React.createElement(DirectoryNode, {
                key: dir.path, dir, depth: 0,
                expandedDirs, onToggle, selectedFile, onSelectFile,
            })
        ),
        sortedFiles(tree).map(f =>
            React.createElement(FileNode, {
                key: f.path, file: f, depth: 0,
                isSelected: selectedFile === f.path,
                onSelect: () => onSelectFile(f.path),
            })
        )
    );
}

// ── FilesView ─────────────────────────────────────────────────────────────────

function FilesView({ tree, expandedDirs, onToggle, selectedFile, onSelectFile, selectedFileInfo, onNavigate, showDatabases }) {
    return React.createElement('div', { className: 'flex flex-1 overflow-hidden' },
        React.createElement('div', {
            className: 'w-72 flex-shrink-0 border-r border-[var(--border)] bg-[var(--panel)] overflow-y-auto'
        },
            React.createElement(FileTree, {
                tree, expandedDirs, onToggle, selectedFile, onSelectFile,
            })
        ),
        React.createElement('div', { className: 'flex-1 overflow-y-auto bg-[var(--panel)]' },
            React.createElement(FileDetails, { file: selectedFileInfo, onNavigate, showDatabases })
        )
    );
}

// ── Routes view components ────────────────────────────────────────────────────

function ConnectionRow({ call, files, onNavigateToFile }) {
    const target = call.to_route || getCallTarget(call, files);
    const isMatched = !!call.to_route;
    const confCls = CONFIDENCE_CLASS[call.confidence] || 'bg-[var(--panel-alt)] text-[var(--fg-dim)]';

    return React.createElement('div', { className: 'flex items-center gap-2 py-1 text-sm flex-wrap' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] select-none' }, '•'),
        React.createElement(ClickableFile, { path: call.from_file, onNavigate: onNavigateToFile }),
        React.createElement('span', { className: isMatched ? 'text-[var(--fg-muted)]' : 'text-[var(--fg-muted)]' }, '→'),
        target
            ? React.createElement('span', {
                className: 'font-mono ' + (isMatched ? 'text-[var(--fg-dim)]' : 'text-[var(--fg-muted)] italic')
              }, target + (isMatched ? '' : ' (no matching route)'))
            : React.createElement('span', { className: 'text-[var(--fg-muted)] italic text-xs' }, '(unknown route)'),
        call.method
            ? React.createElement(MethodBadge, { method: call.method })
            : React.createElement(MethodBadge, { method: 'GET' }),
        call.confidence && React.createElement('span', {
            className: 'text-xs px-1.5 py-0.5 rounded ' + confCls
        }, call.confidence)
    );
}

function ConnectionsSection({ apiCalls, files, onNavigateToFile, title, emptyMessage }) {
    return React.createElement(DetailSection, { title: title || 'Connections', count: apiCalls.length },
        apiCalls.length > 0
            ? apiCalls.map((call, i) =>
                React.createElement(ConnectionRow, { key: i, call, files, onNavigateToFile }))
            : React.createElement('p', { className: 'text-sm text-[var(--fg-muted)] italic' },
                emptyMessage || 'No frontend-to-backend calls detected. Add a fetch call to a page to see connections here.')
    );
}

function PageRow({ page, onNavigateToFile }) {
    return React.createElement('div', { className: 'flex items-center gap-3 py-1 text-sm' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-[var(--fg)] w-48 flex-shrink-0' }, page.path),
        React.createElement(ClickableFile, { path: page.file, onNavigate: onNavigateToFile })
    );
}

function PagesSection({ pages, onNavigateToFile }) {
    return React.createElement(DetailSection, { title: 'Pages', count: pages.length },
        pages.length > 0
            ? pages.map((page, i) =>
                React.createElement(PageRow, { key: i, page, onNavigateToFile }))
            : React.createElement('p', { className: 'text-sm text-[var(--fg-muted)] italic' }, 'No pages found.')
    );
}

function RouteRow({ route, onNavigateToFile }) {
    return React.createElement('div', { className: 'flex items-center gap-2 py-1 text-sm flex-wrap' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-[var(--fg)] w-48 flex-shrink-0' }, route.path),
        ...(route.methods || []).map(m => React.createElement(MethodBadge, { key: m, method: m })),
        React.createElement(ClickableFile, { path: route.file, onNavigate: onNavigateToFile })
    );
}

function APIRoutesSection({ routes, onNavigateToFile }) {
    return React.createElement(DetailSection, { title: 'API Routes', count: routes.length },
        routes.length > 0
            ? routes.map((route, i) =>
                React.createElement(RouteRow, { key: i, route, onNavigateToFile }))
            : React.createElement('p', { className: 'text-sm text-[var(--fg-muted)] italic' }, 'No API routes in this project. API routes live in app/api/*/route.ts files.')
    );
}

function AppRow({ app, onNavigateToFile }) {
    return React.createElement('div', { className: 'flex items-center gap-3 py-1 text-sm' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-[var(--fg)] w-48 flex-shrink-0' }, app.name),
        React.createElement(ClickableFile, { path: app.file, onNavigate: onNavigateToFile })
    );
}

function FastAPIAppsSection({ apps, onNavigateToFile }) {
    return React.createElement(DetailSection, { title: 'FastAPI Apps', count: apps.length },
        apps.length > 0
            ? apps.map((app, i) => React.createElement(AppRow, { key: i, app, onNavigateToFile }))
            : React.createElement('p', { className: 'text-sm text-[var(--fg-muted)] italic' }, 'No FastAPI apps detected.')
    );
}

function RouterRow({ router, onNavigateToFile }) {
    return React.createElement('div', { className: 'flex items-center gap-2 py-1 text-sm flex-wrap' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-[var(--fg)] w-48 flex-shrink-0' }, router.prefix || router.name),
        React.createElement(ReachableBadge, { reachable: router.reachable }),
        React.createElement(ClickableFile, { path: router.file, onNavigate: onNavigateToFile })
    );
}

function RoutersSection({ routers, onNavigateToFile }) {
    return React.createElement(DetailSection, { title: 'Routers', count: routers.length },
        routers.length > 0
            ? routers.map((router, i) => React.createElement(RouterRow, { key: i, router, onNavigateToFile }))
            : React.createElement('p', { className: 'text-sm text-[var(--fg-muted)] italic' }, 'No routers detected.')
    );
}

function FastAPIRouteRow({ route, onNavigateToFile }) {
    return React.createElement('div', { className: 'flex items-center gap-2 py-1 text-sm flex-wrap' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] select-none' }, '•'),
        React.createElement(MethodBadge, { method: route.method }),
        React.createElement('span', { className: 'font-mono text-[var(--fg)]' }, route.path),
        route.is_async && React.createElement('span', { className: 'text-xs px-1.5 py-0.5 rounded bg-indigo-50 text-indigo-500' }, 'async'),
        React.createElement('span', { className: 'text-[var(--fg-muted)]' }, route.handler),
        React.createElement(ReachableBadge, { reachable: route.reachable }),
        React.createElement(ClickableFile, { path: route.file, onNavigate: onNavigateToFile })
    );
}

function FastAPIRoutesSection({ routes, onNavigateToFile }) {
    return React.createElement(DetailSection, { title: 'FastAPI Routes', count: routes.length },
        routes.length > 0
            ? routes.map((route, i) => React.createElement(FastAPIRouteRow, { key: i, route, onNavigateToFile }))
            : React.createElement('p', { className: 'text-sm text-[var(--fg-muted)] italic' }, 'No FastAPI routes detected.')
    );
}

function DatabaseTableRow({ table, orm, onNavigateToFile }) {
    return React.createElement('div', { className: 'flex items-center gap-2 py-1 text-sm flex-wrap' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-[var(--fg)] w-48 flex-shrink-0' }, table.name),
        React.createElement('span', { className: 'text-xs px-1.5 py-0.5 rounded bg-amber-50 text-amber-700' }, orm),
        React.createElement(ClickableFile, { path: table.file, onNavigate: onNavigateToFile })
    );
}

function DatabaseTablesSection({ databases, onNavigateToFile }) {
    const rows = [];
    for (const db of databases) {
        for (const table of (db.tables || [])) rows.push({ table, orm: db.type });
    }
    return React.createElement(DetailSection, { title: 'Database Tables', count: rows.length },
        rows.length > 0
            ? rows.map((r, i) => React.createElement(DatabaseTableRow, { key: i, table: r.table, orm: r.orm, onNavigateToFile }))
            : React.createElement('p', { className: 'text-sm text-[var(--fg-muted)] italic' }, 'No tables detected.')
    );
}

function DatabaseQueryRow({ query, onNavigateToFile }) {
    const confCls = CONFIDENCE_CLASS[query.confidence] || 'bg-[var(--panel-alt)] text-[var(--fg-dim)]';
    return React.createElement('div', { className: 'flex items-center gap-2 py-1 text-sm flex-wrap' },
        React.createElement('span', { className: 'text-[var(--fg-muted)] select-none' }, '•'),
        React.createElement(ClickableFile, { path: query.file, onNavigate: onNavigateToFile }),
        React.createElement('span', { className: 'text-[var(--fg-muted)]' }, '→'),
        React.createElement('span', { className: 'font-mono text-[var(--fg-dim)]' }, query.orm + '.' + query.table),
        React.createElement('span', { className: 'text-xs px-1.5 py-0.5 rounded bg-amber-50 text-amber-700 font-mono' }, query.operation.toUpperCase()),
        React.createElement('span', { className: 'text-xs px-1.5 py-0.5 rounded ' + confCls }, query.confidence)
    );
}

function DatabaseQueriesSection({ files, onNavigateToFile }) {
    const rows = [];
    for (const file of files) {
        for (const query of (file.database_queries || [])) rows.push({ ...query, file: file.path });
    }
    return React.createElement(DetailSection, { title: 'Database Queries', count: rows.length },
        rows.length > 0
            ? rows.map((r, i) => React.createElement(DatabaseQueryRow, { key: i, query: r, onNavigateToFile }))
            : React.createElement('p', { className: 'text-sm text-[var(--fg-muted)] italic' }, 'No database queries detected.')
    );
}

function RoutesView({ analysis, onNavigateToFile }) {
    const nextjs    = analysis.frameworks && analysis.frameworks.nextjs;
    const fastapi   = analysis.frameworks && analysis.frameworks.fastapi;
    const databases = analysis.databases || [];
    if (!nextjs && !fastapi && databases.length === 0) {
        return React.createElement('div', {
            className: 'flex-1 flex items-center justify-center p-8 text-[var(--fg-muted)] text-sm text-center'
        }, 'No framework detected. Routes view is currently available for Next.js and FastAPI projects.');
    }

    return React.createElement('div', { className: 'flex-1 overflow-y-auto bg-[var(--panel)]' },
        React.createElement('div', { className: 'p-6 max-w-3xl' },
            nextjs && React.createElement(ConnectionsSection, {
                apiCalls: nextjs.api_calls || [], files: analysis.files, onNavigateToFile,
            }),
            nextjs && React.createElement(PagesSection, { pages: nextjs.pages || [], onNavigateToFile }),
            nextjs && React.createElement(APIRoutesSection, { routes: nextjs.routes || [], onNavigateToFile }),
            fastapi && React.createElement(FastAPIAppsSection, { apps: fastapi.apps || [], onNavigateToFile }),
            fastapi && React.createElement(RoutersSection, { routers: fastapi.routers || [], onNavigateToFile }),
            fastapi && React.createElement(FastAPIRoutesSection, { routes: fastapi.routes || [], onNavigateToFile }),
            fastapi && React.createElement(ConnectionsSection, {
                apiCalls: (fastapi.api_calls || []).map(c => ({ from_file: c.file, to_route: c.to_route, method: c.method, confidence: c.confidence, line: c.line })),
                files: analysis.files, onNavigateToFile,
                title: 'FastAPI Connections',
                emptyMessage: 'No client-to-server calls detected in this project.',
            }),
            databases.length > 0 && React.createElement(DatabaseTablesSection, { databases, onNavigateToFile }),
            databases.length > 0 && React.createElement(DatabaseQueriesSection, { files: analysis.files || [], onNavigateToFile }),
        )
    );
}

// ── GraphView ─────────────────────────────────────────────────────────────────

function GraphView({ analysis, onNavigateToFile, theme }) {
    const containerRef = React.useRef(null);
    const cyRef        = React.useRef(null);
    const [showApiCalls, setShowApiCalls] = useState(true);
    const [showImports,  setShowImports]  = useState(false);
    const [showUsage,    setShowUsage]    = useState(false);
    const [showRouterInclusion, setShowRouterInclusion] = useState(true);
    const [showQueries, setShowQueries] = useState(true);
    const [nodeTypeFilter, setNodeTypeFilter] = useState({
        page: true, route: true, component: true, router: true, table: true,
    });

    const nextjs    = analysis.frameworks && analysis.frameworks.nextjs;
    const fastapi   = analysis.frameworks && analysis.frameworks.fastapi;
    const databases = analysis.databases || [];
    const hasData = !!(nextjs && (
        (nextjs.pages      || []).length > 0 ||
        (nextjs.routes     || []).length > 0 ||
        (nextjs.components || []).length > 0
    )) || !!(fastapi && (
        (fastapi.apps    || []).length > 0 ||
        (fastapi.routers || []).length > 0 ||
        (fastapi.routes  || []).length > 0
    )) || databases.length > 0;

    // Initialize cytoscape; re-run only if analysis changes.
    useEffect(() => {
        if (!containerRef.current || !hasData) return;

        if (typeof cytoscape === 'undefined') {
            containerRef.current.textContent = 'cytoscape.js failed to load. Check network connectivity.';
            return;
        }

        const graphData = buildGraphData(analysis);

        const cy = cytoscape({
            container: containerRef.current,
            elements: [...graphData.nodes, ...graphData.edges],
            style: buildCytoscapeStyle(getThemeColors()),
            layout: coseLayoutOptions,
        });

        // cose layout is synchronous — all node positions are final here.
        // Hide toggleable edge types so layout benefits from their constraints
        // but they don't clutter the initial view.
        cy.edges('[type="import"]').style('display', 'none');
        cy.edges('[type="usage"]').style('display', 'none');

        // Single click: highlight node and its immediate neighborhood; fade rest.
        cy.on('tap', 'node', evt => {
            const node = evt.target;
            const neighborhood = node.connectedEdges().connectedNodes().union(node);
            cy.elements().addClass('faded').removeClass('highlighted');
            neighborhood.removeClass('faded');
            node.connectedEdges().removeClass('faded');
            node.addClass('highlighted');
        });

        // Click on background: clear all highlight state.
        cy.on('tap', evt => {
            if (evt.target === cy) {
                cy.elements().removeClass('faded').removeClass('highlighted');
            }
        });

        // Double-click: open in Files view.
        cy.on('dbltap', 'node', evt => {
            const file = evt.target.data('file');
            if (file) onNavigateToFile(file);
        });

        cyRef.current = cy;
        return () => { cy.destroy(); cyRef.current = null; };
    }, [analysis]);

    // Sync edge visibility with toggle state.
    useEffect(() => {
        if (!cyRef.current) return;
        cyRef.current.edges('[type="api_call"]').style('display', showApiCalls ? 'element' : 'none');
        cyRef.current.edges('[type="import"]').style('display',   showImports  ? 'element' : 'none');
        cyRef.current.edges('[type="usage"]').style('display',    showUsage    ? 'element' : 'none');
        cyRef.current.edges('[type="router_inclusion"]').style('display', showRouterInclusion ? 'element' : 'none');
        cyRef.current.edges('[type="query"]').style('display', showQueries ? 'element' : 'none');
    }, [showApiCalls, showImports, showUsage, showRouterInclusion, showQueries]);

    // Re-apply node/edge colors when the theme changes. No-op while the
    // graph hasn't mounted yet — the init effect above already builds its
    // style fresh from getThemeColors() at that point, so there's nothing
    // to catch up on.
    useEffect(() => {
        if (!cyRef.current) return;
        cyRef.current.style(buildCytoscapeStyle(getThemeColors())).update();
    }, [theme]);

    // Sync node visibility with type filter.
    useEffect(() => {
        if (!cyRef.current) return;
        for (const [type, visible] of Object.entries(nodeTypeFilter)) {
            cyRef.current.nodes('[type="' + type + '"]').style('display', visible ? 'element' : 'none');
        }
    }, [nodeTypeFilter]);

    if (!hasData) {
        return React.createElement('div', {
            className: 'flex-1 flex items-center justify-center p-8 text-[var(--fg-muted)] text-sm text-center'
        }, 'No graph data available. The graph view shows pages, routes, and components from Next.js and FastAPI projects. Add files to your project to populate it.');
    }

    return React.createElement('div', { className: 'flex-1 flex flex-col bg-[var(--bg)] overflow-hidden' },
        // Controls bar
        React.createElement('div', {
            className: 'flex items-center gap-4 px-4 py-2 bg-[var(--panel)] border-b border-[var(--border)] text-xs flex-shrink-0 flex-wrap'
        },
            React.createElement('div', { className: 'flex items-center gap-2' },
                React.createElement('span', { className: 'text-[var(--fg-muted)] font-medium' }, 'Edges:'),
                makeToggle('API calls', 'var(--edge-api)', showApiCalls, () => setShowApiCalls(v => !v)),
                makeToggle('Imports',   'var(--edge-import)', showImports,  () => setShowImports(v => !v)),
                makeToggle('Usage',     'var(--edge-usage)', showUsage,    () => setShowUsage(v => !v)),
                makeToggle('Router inclusion', 'var(--edge-router)', showRouterInclusion, () => setShowRouterInclusion(v => !v)),
                databases.length > 0 && makeToggle('Queries', 'var(--edge-query)', showQueries, () => setShowQueries(v => !v)),
            ),
            React.createElement('div', { className: 'flex items-center gap-2 ml-4' },
                React.createElement('span', { className: 'text-[var(--fg-muted)] font-medium' }, 'Show:'),
                makeToggle('Pages',      'var(--node-page)', nodeTypeFilter.page,
                    () => setNodeTypeFilter(p => ({ ...p, page:      !p.page }))),
                makeToggle('Routes',     'var(--node-route)', nodeTypeFilter.route,
                    () => setNodeTypeFilter(p => ({ ...p, route:     !p.route }))),
                makeToggle('Components', 'var(--node-component)', nodeTypeFilter.component,
                    () => setNodeTypeFilter(p => ({ ...p, component: !p.component }))),
                makeToggle('Routers',    'var(--node-router)', nodeTypeFilter.router,
                    () => setNodeTypeFilter(p => ({ ...p, router:    !p.router }))),
                databases.length > 0 && makeToggle('Tables', 'var(--node-table)', nodeTypeFilter.table,
                    () => setNodeTypeFilter(p => ({ ...p, table: !p.table }))),
            ),
            React.createElement('div', { className: 'ml-auto text-[var(--fg-muted)] text-xs' },
                'Click to highlight · Double-click to open in Files'
            ),
        ),
        // Graph canvas — flex-1 + minHeight:0 lets it fill remaining vertical space.
        React.createElement('div', {
            ref: containerRef,
            className: 'flex-1',
            style: { minHeight: 0 }
        })
    );
}

// ── Tabs ──────────────────────────────────────────────────────────────────────

function Tabs({ currentView, onSwitchView }) {
    const tabs = [
        { id: 'files',  label: 'Files',  enabled: true },
        { id: 'routes', label: 'Routes', enabled: true },
        { id: 'graph',  label: 'Graph',  enabled: true },
    ];
    return React.createElement('div', { className: 'flex border-b border-[var(--border)] bg-[var(--panel)] flex-shrink-0 px-4' },
        tabs.map(tab => {
            const isActive = tab.id === currentView;
            let cls = 'px-4 py-2 text-sm border-b-2 -mb-px transition-colors ';
            if (!tab.enabled) {
                cls += 'border-transparent text-[var(--fg-muted)] cursor-not-allowed';
            } else if (isActive) {
                cls += 'border-blue-500 text-blue-600 font-medium';
            } else {
                cls += 'border-transparent text-[var(--fg-muted)] hover:text-[var(--fg-dim)] hover:bg-[var(--bg)] cursor-pointer';
            }
            return React.createElement('button', {
                key: tab.id,
                className: cls,
                disabled: !tab.enabled,
                onClick: tab.enabled && !isActive ? () => onSwitchView(tab.id) : undefined,
            }, tab.label);
        })
    );
}

// ── Header ────────────────────────────────────────────────────────────────────

function Header({ analysis, lastUpdate, themeMode, onSelectTheme }) {
    const { project, files } = analysis;
    const fw = project.frameworks || [];
    const [justUpdated, setJustUpdated] = useState(false);

    useEffect(() => {
        if (!lastUpdate) return;
        setJustUpdated(true);
        const t = setTimeout(() => setJustUpdated(false), 3000);
        return () => clearTimeout(t);
    }, [lastUpdate]);

    return React.createElement('div', {
        className: 'bg-[var(--panel)] border-b border-[var(--border)] px-5 py-2.5 flex items-center gap-3 flex-shrink-0'
    },
        React.createElement('span', { className: 'font-mono font-bold text-[var(--fg)]' }, project.name),
        fw.map(f => React.createElement('span', {
            key: f,
            className: 'bg-indigo-50 text-indigo-600 text-xs px-2 py-0.5 rounded font-mono'
        }, f)),
        justUpdated && React.createElement('span', {
            className: 'text-xs px-2 py-0.5 rounded bg-green-50 text-green-600 font-medium'
        }, 'Just updated'),
        React.createElement('span', { className: 'ml-auto text-xs text-[var(--fg-muted)]' },
            files.length + ' ' + (files.length === 1 ? 'file' : 'files')),
        React.createElement('div', { className: 'theme-switcher', role: 'radiogroup', 'aria-label': 'Theme' },
            THEME_MODES.map(mode => React.createElement('button', {
                key: mode,
                className: 'theme-option' + (mode === themeMode ? ' active' : ''),
                'aria-label': mode + ' theme',
                title: mode.charAt(0).toUpperCase() + mode.slice(1),
                onClick: () => onSelectTheme(mode),
            }, mode.charAt(0).toUpperCase() + mode.slice(1)))
        )
    );
}

// ── App ───────────────────────────────────────────────────────────────────────

function App() {
    const [analysis,     setAnalysis]     = useState(null);
    const [selectedFile, setSelectedFile] = useState(null);
    const [expandedDirs, setExpandedDirs] = useState(new Set());
    const [currentView,  setCurrentView]  = useState('files');
    const [error,        setError]        = useState(null);
    const [lastUpdate,   setLastUpdate]   = useState(null);

    const [themeMode, setThemeMode] = useState(() => {
        const stored = localStorage.getItem(THEME_STORAGE_KEY);
        return THEME_MODES.includes(stored) ? stored : 'light';
    });
    const [systemDark, setSystemDark] = useState(
        !!(window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches)
    );
    const effectiveTheme = themeMode === 'auto' ? (systemDark ? 'dark' : 'light') : themeMode;

    useEffect(() => {
        if (!window.matchMedia) return;
        const mq = window.matchMedia('(prefers-color-scheme: dark)');
        const handler = e => setSystemDark(e.matches);
        mq.addEventListener('change', handler);
        return () => mq.removeEventListener('change', handler);
    }, []);

    useEffect(() => {
        localStorage.setItem(THEME_STORAGE_KEY, themeMode);
    }, [themeMode]);

    useEffect(() => {
        if (effectiveTheme === 'dark') {
            document.documentElement.setAttribute('data-theme', 'dark');
        } else {
            document.documentElement.removeAttribute('data-theme');
        }
    }, [effectiveTheme]);

    const wsRef              = useRef(null);
    const reconnectTimerRef  = useRef(null);
    const reconnectDelayRef  = useRef(1000);

    function fetchAnalysis() {
        fetch('/api/analysis')
            .then(r => r.json())
            .then(data => {
                setAnalysis(data);
                const top = new Set();
                data.files.forEach(f => {
                    const slash = f.path.indexOf('/');
                    if (slash > 0) top.add(f.path.slice(0, slash));
                });
                setExpandedDirs(top);
            })
            .catch(e => setError(e.message));
    }

    useEffect(() => {
        fetchAnalysis();
    }, []);

    useEffect(() => {
        connectWebSocket();
        return () => {
            if (reconnectTimerRef.current) clearTimeout(reconnectTimerRef.current);
            if (wsRef.current) wsRef.current.close();
        };
    }, []);

    function connectWebSocket() {
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const ws = new WebSocket(`${protocol}//${window.location.host}/ws`);

        ws.onopen = () => {
            reconnectDelayRef.current = 1000;
        };

        ws.onmessage = (e) => {
            try {
                const msg = JSON.parse(e.data);
                if (msg.type === 'analysis_updated') {
                    fetch('/api/analysis')
                        .then(r => r.json())
                        .then(data => {
                            setAnalysis(data);
                            setLastUpdate(Date.now());
                        })
                        .catch(err => console.error('re-fetch failed:', err));
                }
            } catch (err) {
                console.error('ws message parse error:', err);
            }
        };

        ws.onclose = () => {
            const delay = reconnectDelayRef.current;
            reconnectTimerRef.current = setTimeout(() => {
                reconnectDelayRef.current = Math.min(delay * 2, 30000);
                connectWebSocket();
            }, delay);
        };

        wsRef.current = ws;
    }

    function handleToggle(dirPath) {
        setExpandedDirs(prev => {
            const next = new Set(prev);
            if (next.has(dirPath)) next.delete(dirPath); else next.add(dirPath);
            return next;
        });
    }

    // Navigate within Files view (e.g., clicking a resolved import).
    function handleNavigate(resolvedPath) {
        setSelectedFile(resolvedPath);
        setExpandedDirs(prev => {
            const next = new Set(prev);
            const parts = resolvedPath.split('/');
            for (let i = 1; i < parts.length; i++) next.add(parts.slice(0, i).join('/'));
            return next;
        });
    }

    // Navigate from Routes or Graph view to Files view with a specific file selected.
    function handleNavigateToFile(filePath) {
        setCurrentView('files');
        setSelectedFile(filePath);
        setExpandedDirs(prev => {
            const next = new Set(prev);
            const parts = filePath.split('/');
            for (let i = 1; i < parts.length; i++) next.add(parts.slice(0, i).join('/'));
            return next;
        });
    }

    if (error) {
        return React.createElement('div', { className: 'p-8 text-red-600 font-mono text-sm' },
            'Error loading analysis: ' + error);
    }
    if (!analysis) {
        return React.createElement('div', { className: 'p-8 text-[var(--fg-muted)] text-sm' }, 'Loading...');
    }

    const tree = buildTree(analysis.files);
    const selectedFileInfo = analysis.files.find(f => f.path === selectedFile) || null;
    const showDatabases = !!(analysis.databases && analysis.databases.length > 0);

    return React.createElement('div', { className: 'h-screen flex flex-col bg-[var(--bg)]' },
        React.createElement(Header, { analysis, lastUpdate, themeMode, onSelectTheme: setThemeMode }),
        React.createElement(Tabs, { currentView, onSwitchView: setCurrentView }),
        currentView === 'files'
            ? React.createElement(FilesView, {
                tree, expandedDirs,
                onToggle: handleToggle,
                selectedFile,
                onSelectFile: setSelectedFile,
                selectedFileInfo,
                onNavigate: handleNavigate,
                showDatabases,
            })
            : currentView === 'routes'
                ? React.createElement(RoutesView, {
                    analysis,
                    onNavigateToFile: handleNavigateToFile,
                })
                : React.createElement(GraphView, {
                    analysis,
                    onNavigateToFile: handleNavigateToFile,
                    theme: effectiveTheme,
                })
    );
}

ReactDOM.createRoot(document.getElementById('root')).render(React.createElement(App));
