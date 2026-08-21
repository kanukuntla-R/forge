// render.js — loads a bundled component in headless Chromium and screenshots it.
//
// Usage: node render.js <bundle.js> <output.png> <viewportWidth> <viewportHeight>
const { chromium } = require('playwright');
const fs = require('fs');

const [bundlePath, outputPath, viewportW, viewportH] = process.argv.slice(2);

const bundle = fs.readFileSync(bundlePath, 'utf-8');

const html = `<!DOCTYPE html>
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

(async () => {
    const browser = await chromium.launch();
    try {
        const context = await browser.newContext({
            viewport: { width: parseInt(viewportW, 10), height: parseInt(viewportH, 10) },
        });
        const page = await context.newPage();
        await page.setContent(html);
        await page.waitForLoadState('networkidle');
        await page.screenshot({ path: outputPath });
    } finally {
        await browser.close();
    }
})().catch(err => {
    console.error(err.message || String(err));
    process.exit(1);
});
