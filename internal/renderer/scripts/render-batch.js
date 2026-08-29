// render-batch.js — screenshots many pre-bundled components using ONE
// Playwright browser (N concurrent BrowserContexts, not N browsers).
//
// Usage: node render-batch.js <manifest.json>
// manifest.json: { parallel, viewport: {width, height}, items: [{ outName, bundlePath, outputPath }] }
// Prints a JSON array of { name, status, time, error? } to stdout.
const { chromium } = require('playwright');
const fs = require('fs');
const pLimit = require('p-limit');

const [manifestPath] = process.argv.slice(2);
const manifest = JSON.parse(fs.readFileSync(manifestPath, 'utf-8'));
const { items, parallel, viewport } = manifest;

function htmlFor(bundlePath) {
    const bundle = fs.readFileSync(bundlePath, 'utf-8');
    return `<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8" />
<title>Component Preview</title>
<style>body { margin: 0; padding: 20px; font-family: system-ui; }</style>
</head>
<body>
<div id="root"></div>
<script>${bundle}</script>
</body>
</html>`;
}

(async () => {
    const browser = await chromium.launch();
    const limit = pLimit(parallel || 4);

    const results = await Promise.all(items.map(item => limit(async () => {
        const start = Date.now();
        try {
            const context = await browser.newContext({
                viewport: { width: viewport.width, height: viewport.height },
            });
            const page = await context.newPage();
            await page.setContent(htmlFor(item.bundlePath));
            await page.waitForLoadState('networkidle');
            await page.screenshot({ path: item.outputPath });
            await context.close();
            return { name: item.outName, status: 'success', time: Date.now() - start };
        } catch (err) {
            return { name: item.outName, status: 'failed', time: Date.now() - start, error: err.message || String(err) };
        }
    })));

    await browser.close();
    console.log(JSON.stringify(results));
})().catch(err => {
    console.error(err.message || String(err));
    process.exit(1);
});
