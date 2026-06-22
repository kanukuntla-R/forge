const { useState, useEffect } = React;

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
    const nextjs = analysis.frameworks && analysis.frameworks.nextjs;
    if (!nextjs) return { nodes: [], edges: [] };

    const nodes   = [];
    const edges   = [];
    const nodeIds = new Set();

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

    // API call edges — only emit if both endpoints are graph nodes
    for (let i = 0; i < (nextjs.api_calls || []).length; i++) {
        const call = nextjs.api_calls[i];
        if (!call.to_route) continue;
        const targetRoute = (nextjs.routes || []).find(r => r.path === call.to_route);
        if (!targetRoute) continue;
        if (!nodeIds.has(call.from_file) || !nodeIds.has(targetRoute.file)) continue;
        edges.push({
            data: {
                id: 'api-' + i,
                source: call.from_file,
                target: targetRoute.file,
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

    return { nodes, edges };
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

const cytoscapeStyle = [
    {
        selector: 'node[type="page"]',
        style: {
            'shape':        'round-rectangle',
            'background-color': '#3b82f6',
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
            'background-color': '#10b981',
            'label':        'data(label)',
            'color':        '#fff',
            'text-valign':  'center',
            'text-halign':  'center',
            'font-size':    '10px',
            'font-family':  'monospace',
            'width':        80,
            'height':       50,
            'border-width': 0,
        }
    },
    {
        selector: 'node[type="component"]',
        style: {
            'shape':        'ellipse',
            'background-color': '#8b5cf6',
            'label':        'data(label)',
            'color':        '#fff',
            'text-valign':  'center',
            'text-halign':  'center',
            'font-size':    '10px',
            'font-family':  'monospace',
            'width':        60,
            'height':       40,
            'border-width': 0,
        }
    },
    {
        selector: 'edge[type="api_call"]',
        style: {
            'width':                  2,
            'line-color':             '#3b82f6',
            'target-arrow-color':     '#3b82f6',
            'target-arrow-shape':     'triangle',
            'curve-style':            'bezier',
            'label':                  'data(method)',
            'font-size':              '9px',
            'color':                  '#3b82f6',
            'text-background-color':  '#fff',
            'text-background-opacity': 1,
            'text-background-padding': '2px',
        }
    },
    {
        selector: 'edge[type="import"]',
        style: {
            'width':              1,
            'line-color':         '#9ca3af',
            'target-arrow-color': '#9ca3af',
            'target-arrow-shape': 'triangle',
            'curve-style':        'bezier',
            'line-style':         'dashed',
        }
    },
    {
        selector: 'edge[type="usage"]',
        style: {
            'width':              1.5,
            'line-color':         '#a78bfa',
            'target-arrow-color': '#a78bfa',
            'target-arrow-shape': 'triangle',
            'curve-style':        'bezier',
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

// ── Shared primitives ─────────────────────────────────────────────────────────

function MethodBadge({ method }) {
    const cls = METHOD_CLASS[method] || 'bg-gray-100 text-gray-600';
    return React.createElement('span', {
        className: 'text-xs px-1.5 py-0.5 rounded font-mono font-medium ' + cls
    }, method);
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
            className: 'text-xs font-semibold uppercase tracking-wide text-gray-400 mb-1.5'
        }, title + ' (' + count + ')'),
        children
    );
}

function makeToggle(label, color, active, onClick) {
    return React.createElement('button', {
        onClick,
        className: 'flex items-center gap-1.5 px-2 py-1 rounded text-xs cursor-pointer transition-colors ' +
            (active ? 'bg-gray-100' : 'opacity-50 hover:opacity-75'),
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
        React.createElement('span', { className: 'text-gray-300 mt-0.5 select-none' }, '•'),
        React.createElement('div', { className: 'min-w-0' },
            React.createElement('span', { className: 'font-mono text-gray-700' }, imp.source),
            React.createElement('span', { className: 'text-gray-400 mx-1.5' }, '·'),
            React.createElement('span', { className: 'text-gray-600' }, names),
            imp.external
                ? React.createElement('span', {
                    className: 'ml-2 bg-gray-100 text-gray-400 text-xs px-1.5 py-0.5 rounded'
                  }, 'ext')
                : imp.resolved
                    ? React.createElement('span', { className: 'ml-2 text-gray-400' },
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
        React.createElement('span', { className: 'text-gray-300 select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-gray-700' }, exp.name),
        React.createElement('span', { className: 'bg-gray-100 text-gray-500 text-xs px-1.5 py-0.5 rounded' }, exp.type),
        React.createElement('span', { className: 'text-gray-300 text-xs' }, 'line ' + exp.line)
    );
}

function DeclarationRow({ decl }) {
    return React.createElement('div', { className: 'flex items-center gap-2 py-0.5 text-sm' },
        React.createElement('span', { className: 'text-gray-300 select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-gray-700' }, decl.name),
        React.createElement('span', { className: 'bg-gray-100 text-gray-500 text-xs px-1.5 py-0.5 rounded' }, decl.type),
        React.createElement('span', { className: 'text-gray-300 text-xs' }, 'line ' + decl.line)
    );
}

function CallRow({ call }) {
    const cls = CONFIDENCE_CLASS[call.confidence] || 'bg-gray-100 text-gray-600';
    return React.createElement('div', { className: 'flex items-center gap-2 py-0.5 text-sm' },
        React.createElement('span', { className: 'text-gray-300 select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-gray-700' }, call.target),
        call.method && React.createElement(MethodBadge, { method: call.method }),
        React.createElement('span', { className: 'text-xs px-1.5 py-0.5 rounded ' + cls }, call.confidence)
    );
}

// ── FileDetails ───────────────────────────────────────────────────────────────

function FileDetails({ file, onNavigate }) {
    if (!file) {
        return React.createElement('div', {
            className: 'flex-1 flex items-center justify-center text-gray-400 text-sm'
        }, 'Select a file to view details');
    }

    const imports      = file.imports      || [];
    const exports_     = file.exports      || [];
    const declarations = file.declarations || [];
    const calls        = file.calls        || [];
    const none = React.createElement('p', { className: 'text-sm text-gray-400 italic' }, 'None');

    return React.createElement('div', { className: 'p-6 max-w-3xl' },
        React.createElement('h2', { className: 'font-mono font-semibold text-base mb-1 break-all' }, file.path),
        React.createElement('div', { className: 'flex gap-4 text-xs text-gray-400 mb-5 pb-4 border-b' },
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
        )
    );
}

// ── File tree components ──────────────────────────────────────────────────────

function FileNode({ file, depth, isSelected, onSelect }) {
    return React.createElement('div', {
        className: 'flex items-center py-0.5 cursor-pointer ' +
            (isSelected ? 'bg-blue-100' : 'hover:bg-gray-100'),
        style: { paddingLeft: (depth * 16 + 20) + 'px', paddingRight: '8px' },
        onClick: onSelect,
    },
        React.createElement('span', {
            className: 'font-mono text-sm truncate ' +
                (isSelected ? 'text-blue-700 font-medium' : 'text-gray-600')
        }, file.name)
    );
}

function DirectoryNode({ dir, depth, expandedDirs, onToggle, selectedFile, onSelectFile }) {
    const isExpanded = expandedDirs.has(dir.path);
    const count = countTotalFiles(dir);
    return React.createElement('div', null,
        React.createElement('div', {
            className: 'flex items-center gap-1 py-0.5 cursor-pointer select-none hover:bg-gray-100',
            style: { paddingLeft: (depth * 16 + 4) + 'px', paddingRight: '8px' },
            onClick: () => onToggle(dir.path),
        },
            React.createElement('span', { className: 'text-gray-400 text-xs w-3 flex-shrink-0' },
                isExpanded ? '▾' : '▸'),
            React.createElement('span', { className: 'font-mono text-sm text-gray-700' }, dir.name + '/'),
            React.createElement('span', { className: 'ml-1 text-xs text-gray-400' }, '(' + count + ')')
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
        return React.createElement('div', { className: 'p-4 text-sm text-gray-400 italic' },
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

function FilesView({ tree, expandedDirs, onToggle, selectedFile, onSelectFile, selectedFileInfo, onNavigate }) {
    return React.createElement('div', { className: 'flex flex-1 overflow-hidden' },
        React.createElement('div', {
            className: 'w-72 flex-shrink-0 border-r bg-white overflow-y-auto'
        },
            React.createElement(FileTree, {
                tree, expandedDirs, onToggle, selectedFile, onSelectFile,
            })
        ),
        React.createElement('div', { className: 'flex-1 overflow-y-auto bg-white' },
            React.createElement(FileDetails, { file: selectedFileInfo, onNavigate })
        )
    );
}

// ── Routes view components ────────────────────────────────────────────────────

function ConnectionRow({ call, files, onNavigateToFile }) {
    const target = call.to_route || getCallTarget(call, files);
    const isMatched = !!call.to_route;
    const confCls = CONFIDENCE_CLASS[call.confidence] || 'bg-gray-100 text-gray-600';

    return React.createElement('div', { className: 'flex items-center gap-2 py-1 text-sm flex-wrap' },
        React.createElement('span', { className: 'text-gray-300 select-none' }, '•'),
        React.createElement(ClickableFile, { path: call.from_file, onNavigate: onNavigateToFile }),
        React.createElement('span', { className: isMatched ? 'text-gray-400' : 'text-gray-200' }, '→'),
        target
            ? React.createElement('span', {
                className: 'font-mono ' + (isMatched ? 'text-gray-700' : 'text-gray-400 italic')
              }, target + (isMatched ? '' : ' (no matching route)'))
            : React.createElement('span', { className: 'text-gray-300 italic text-xs' }, '(unknown route)'),
        call.method
            ? React.createElement(MethodBadge, { method: call.method })
            : React.createElement(MethodBadge, { method: 'GET' }),
        call.confidence && React.createElement('span', {
            className: 'text-xs px-1.5 py-0.5 rounded ' + confCls
        }, call.confidence)
    );
}

function ConnectionsSection({ apiCalls, files, onNavigateToFile }) {
    return React.createElement(DetailSection, { title: 'Connections', count: apiCalls.length },
        apiCalls.length > 0
            ? apiCalls.map((call, i) =>
                React.createElement(ConnectionRow, { key: i, call, files, onNavigateToFile }))
            : React.createElement('p', { className: 'text-sm text-gray-400 italic' }, 'No connections found.')
    );
}

function PageRow({ page, onNavigateToFile }) {
    return React.createElement('div', { className: 'flex items-center gap-3 py-1 text-sm' },
        React.createElement('span', { className: 'text-gray-300 select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-gray-800 w-48 flex-shrink-0' }, page.path),
        React.createElement(ClickableFile, { path: page.file, onNavigate: onNavigateToFile })
    );
}

function PagesSection({ pages, onNavigateToFile }) {
    return React.createElement(DetailSection, { title: 'Pages', count: pages.length },
        pages.length > 0
            ? pages.map((page, i) =>
                React.createElement(PageRow, { key: i, page, onNavigateToFile }))
            : React.createElement('p', { className: 'text-sm text-gray-400 italic' }, 'No pages found.')
    );
}

function RouteRow({ route, onNavigateToFile }) {
    return React.createElement('div', { className: 'flex items-center gap-2 py-1 text-sm flex-wrap' },
        React.createElement('span', { className: 'text-gray-300 select-none' }, '•'),
        React.createElement('span', { className: 'font-mono text-gray-800 w-48 flex-shrink-0' }, route.path),
        ...(route.methods || []).map(m => React.createElement(MethodBadge, { key: m, method: m })),
        React.createElement(ClickableFile, { path: route.file, onNavigate: onNavigateToFile })
    );
}

function APIRoutesSection({ routes, onNavigateToFile }) {
    return React.createElement(DetailSection, { title: 'API Routes', count: routes.length },
        routes.length > 0
            ? routes.map((route, i) =>
                React.createElement(RouteRow, { key: i, route, onNavigateToFile }))
            : React.createElement('p', { className: 'text-sm text-gray-400 italic' }, 'No API routes found.')
    );
}

function RoutesView({ analysis, onNavigateToFile }) {
    const nextjs = analysis.frameworks && analysis.frameworks.nextjs;
    if (!nextjs) {
        return React.createElement('div', {
            className: 'flex-1 flex items-center justify-center p-8 text-gray-400 text-sm text-center'
        }, 'No framework detected. Routes view is currently only available for Next.js projects.');
    }

    const apiCalls = nextjs.api_calls || [];
    const pages    = nextjs.pages    || [];
    const routes   = nextjs.routes   || [];

    return React.createElement('div', { className: 'flex-1 overflow-y-auto bg-white' },
        React.createElement('div', { className: 'p-6 max-w-3xl' },
            React.createElement(ConnectionsSection, { apiCalls, files: analysis.files, onNavigateToFile }),
            React.createElement(PagesSection, { pages, onNavigateToFile }),
            React.createElement(APIRoutesSection, { routes, onNavigateToFile })
        )
    );
}

// ── GraphView ─────────────────────────────────────────────────────────────────

function GraphView({ analysis, onNavigateToFile }) {
    const containerRef = React.useRef(null);
    const cyRef        = React.useRef(null);
    const [showApiCalls, setShowApiCalls] = useState(true);
    const [showImports,  setShowImports]  = useState(false);
    const [showUsage,    setShowUsage]    = useState(false);
    const [nodeTypeFilter, setNodeTypeFilter] = useState({
        page: true, route: true, component: true,
    });

    const nextjs  = analysis.frameworks && analysis.frameworks.nextjs;
    const hasData = !!(nextjs && (
        (nextjs.pages      || []).length > 0 ||
        (nextjs.routes     || []).length > 0 ||
        (nextjs.components || []).length > 0
    ));

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
            style: cytoscapeStyle,
            layout: {
                name: 'cose',
                padding: 30,
                nodeRepulsion: 8000,
                idealEdgeLength: 100,
                randomize: false,
            },
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
    }, [showApiCalls, showImports, showUsage]);

    // Sync node visibility with type filter.
    useEffect(() => {
        if (!cyRef.current) return;
        for (const [type, visible] of Object.entries(nodeTypeFilter)) {
            cyRef.current.nodes('[type="' + type + '"]').style('display', visible ? 'element' : 'none');
        }
    }, [nodeTypeFilter]);

    if (!hasData) {
        return React.createElement('div', {
            className: 'flex-1 flex items-center justify-center p-8 text-gray-400 text-sm text-center'
        }, 'No graph data available. The graph view shows pages, API routes, and components from Next.js projects.');
    }

    return React.createElement('div', { className: 'flex-1 flex flex-col bg-gray-50 overflow-hidden' },
        // Controls bar
        React.createElement('div', {
            className: 'flex items-center gap-4 px-4 py-2 bg-white border-b text-xs flex-shrink-0 flex-wrap'
        },
            React.createElement('div', { className: 'flex items-center gap-2' },
                React.createElement('span', { className: 'text-gray-500 font-medium' }, 'Edges:'),
                makeToggle('API calls', '#3b82f6', showApiCalls, () => setShowApiCalls(v => !v)),
                makeToggle('Imports',   '#9ca3af', showImports,  () => setShowImports(v => !v)),
                makeToggle('Usage',     '#a78bfa', showUsage,    () => setShowUsage(v => !v)),
            ),
            React.createElement('div', { className: 'flex items-center gap-2 ml-4' },
                React.createElement('span', { className: 'text-gray-500 font-medium' }, 'Show:'),
                makeToggle('Pages',      '#3b82f6', nodeTypeFilter.page,
                    () => setNodeTypeFilter(p => ({ ...p, page:      !p.page }))),
                makeToggle('Routes',     '#10b981', nodeTypeFilter.route,
                    () => setNodeTypeFilter(p => ({ ...p, route:     !p.route }))),
                makeToggle('Components', '#8b5cf6', nodeTypeFilter.component,
                    () => setNodeTypeFilter(p => ({ ...p, component: !p.component }))),
            ),
            React.createElement('div', { className: 'ml-auto text-gray-400 text-xs' },
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
    return React.createElement('div', { className: 'flex border-b bg-white flex-shrink-0 px-4' },
        tabs.map(tab => {
            const isActive = tab.id === currentView;
            let cls = 'px-4 py-2 text-sm border-b-2 -mb-px transition-colors ';
            if (!tab.enabled) {
                cls += 'border-transparent text-gray-300 cursor-not-allowed';
            } else if (isActive) {
                cls += 'border-blue-500 text-blue-600 font-medium';
            } else {
                cls += 'border-transparent text-gray-500 hover:text-gray-700 hover:bg-gray-50 cursor-pointer';
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

function Header({ analysis }) {
    const { project, files } = analysis;
    const fw = project.frameworks || [];
    return React.createElement('div', {
        className: 'bg-white border-b px-5 py-2.5 flex items-center gap-3 flex-shrink-0'
    },
        React.createElement('span', { className: 'font-mono font-bold text-gray-800' }, project.name),
        fw.map(f => React.createElement('span', {
            key: f,
            className: 'bg-indigo-50 text-indigo-600 text-xs px-2 py-0.5 rounded font-mono'
        }, f)),
        React.createElement('span', { className: 'ml-auto text-xs text-gray-400' },
            files.length + ' ' + (files.length === 1 ? 'file' : 'files'))
    );
}

// ── App ───────────────────────────────────────────────────────────────────────

function App() {
    const [analysis,     setAnalysis]     = useState(null);
    const [selectedFile, setSelectedFile] = useState(null);
    const [expandedDirs, setExpandedDirs] = useState(new Set());
    const [currentView,  setCurrentView]  = useState('files');
    const [error,        setError]        = useState(null);

    useEffect(() => {
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
    }, []);

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
        return React.createElement('div', { className: 'p-8 text-gray-400 text-sm' }, 'Loading...');
    }

    const tree = buildTree(analysis.files);
    const selectedFileInfo = analysis.files.find(f => f.path === selectedFile) || null;

    return React.createElement('div', { className: 'h-screen flex flex-col bg-gray-50' },
        React.createElement(Header, { analysis }),
        React.createElement(Tabs, { currentView, onSwitchView: setCurrentView }),
        currentView === 'files'
            ? React.createElement(FilesView, {
                tree, expandedDirs,
                onToggle: handleToggle,
                selectedFile,
                onSelectFile: setSelectedFile,
                selectedFileInfo,
                onNavigate: handleNavigate,
            })
            : currentView === 'routes'
                ? React.createElement(RoutesView, {
                    analysis,
                    onNavigateToFile: handleNavigateToFile,
                })
                : React.createElement(GraphView, {
                    analysis,
                    onNavigateToFile: handleNavigateToFile,
                })
    );
}

ReactDOM.createRoot(document.getElementById('root')).render(React.createElement(App));
