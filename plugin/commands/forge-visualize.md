---
description: Visualize the current codebase with forge dashboard
---

Run: `forge visualize` in the current project directory.

This starts a dashboard at http://localhost:5050 showing:
- Files, routes, components, database tables
- Cross-language connections (TypeScript to Python)
- Filter toggles for edges and node types

If forge isn't installed, tell the user:
"Install forge with: `curl -fsSL https://raw.githubusercontent.com/kanukuntla-R/forge/main/install.sh | bash`"

Then instruct them to open http://localhost:5050 in a browser to see the visualization.
