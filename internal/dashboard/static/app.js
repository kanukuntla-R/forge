const { useState, useEffect } = React;

function App() {
    const [analysis, setAnalysis] = useState(null);
    const [error, setError] = useState(null);

    useEffect(() => {
        fetch('/api/analysis')
            .then(r => r.json())
            .then(setAnalysis)
            .catch(e => setError(e.message));
    }, []);

    if (error) {
        return React.createElement('div', { className: 'p-8 text-red-600' }, 'Error: ' + error);
    }
    if (!analysis) {
        return React.createElement('div', { className: 'p-8 text-gray-500' }, 'Loading...');
    }

    const fileCount = analysis.files ? analysis.files.length : 0;
    const frameworks = analysis.project && analysis.project.frameworks;

    return React.createElement('div', { className: 'p-8 max-w-4xl mx-auto' },
        React.createElement('h1', { className: 'text-3xl font-bold mb-2' }, 'forge dashboard'),
        React.createElement('p', { className: 'text-sm text-gray-400 mb-6' },
            'M8.5a placeholder — real UI coming in M8.5b'),
        React.createElement('div', { className: 'bg-white p-6 rounded-lg shadow' },
            React.createElement('h2', { className: 'text-xl font-semibold mb-3' },
                analysis.project ? analysis.project.name : '(unknown project)'),
            React.createElement('p', { className: 'text-gray-700' },
                fileCount + ' ' + (fileCount === 1 ? 'file' : 'files') + ' analyzed'),
            frameworks && frameworks.length > 0 &&
                React.createElement('p', { className: 'text-gray-700 mt-1' },
                    'Frameworks: ' + frameworks.join(', '))
        )
    );
}

ReactDOM.createRoot(document.getElementById('root')).render(React.createElement(App));
