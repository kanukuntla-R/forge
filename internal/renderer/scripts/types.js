// types.js — extracts the default-exported React component's prop types.
//
// Usage: node types.js <component.tsx>
// Prints JSON to stdout: { props: [{name, type, optional, shape}], warnings: [string] }
//
// Level 1 scope (see forge M13.1 design): string/number/boolean/undefined,
// object props with primitive fields, functions become a "function"
// placeholder. Anything else (generics, unions, etc.) is reported as
// "unsupported" and logged as a warning — the Go side substitutes a
// placeholder value for those.
const ts = require('typescript');
const fs = require('fs');

const [componentPath] = process.argv.slice(2);
const source = fs.readFileSync(componentPath, 'utf-8');
const sourceFile = ts.createSourceFile(componentPath, source, ts.ScriptTarget.Latest, true, ts.ScriptKind.TSX);

const warnings = [];

function findDefaultComponent(sf) {
    let defaultName = null;
    let fn = null;

    ts.forEachChild(sf, node => {
        // export default function Name(...) {}
        if (ts.isFunctionDeclaration(node) && node.modifiers?.some(m => m.kind === ts.SyntaxKind.DefaultKeyword)) {
            fn = node;
        }
        // export default Name; (function/arrow declared earlier under that name)
        if (ts.isExportAssignment(node) && !node.isExportEquals && ts.isIdentifier(node.expression)) {
            defaultName = node.expression.text;
        }
    });

    if (fn) return fn;
    if (!defaultName) return null;

    let found = null;
    ts.forEachChild(sf, node => {
        if (ts.isFunctionDeclaration(node) && node.name?.text === defaultName) {
            found = node;
        }
        if (ts.isVariableStatement(node)) {
            for (const decl of node.declarationList.declarations) {
                if (ts.isIdentifier(decl.name) && decl.name.text === defaultName &&
                    decl.initializer && (ts.isArrowFunction(decl.initializer) || ts.isFunctionExpression(decl.initializer))) {
                    found = decl.initializer;
                }
            }
        }
    });
    return found;
}

function resolveTypeMembers(typeNode) {
    if (!typeNode) return null;
    if (ts.isTypeLiteralNode(typeNode)) {
        return typeNode.members;
    }
    if (ts.isTypeReferenceNode(typeNode) && ts.isIdentifier(typeNode.typeName)) {
        const name = typeNode.typeName.text;
        let members = null;
        ts.forEachChild(sourceFile, node => {
            if (ts.isInterfaceDeclaration(node) && node.name.text === name) {
                members = node.members;
            }
            if (ts.isTypeAliasDeclaration(node) && node.name.text === name && ts.isTypeLiteralNode(node.type)) {
                members = node.type.members;
            }
        });
        return members;
    }
    return null;
}

function classify(typeNode) {
    if (!typeNode) return { type: 'unsupported' };
    switch (typeNode.kind) {
        case ts.SyntaxKind.StringKeyword:
            return { type: 'string' };
        case ts.SyntaxKind.NumberKeyword:
            return { type: 'number' };
        case ts.SyntaxKind.BooleanKeyword:
            return { type: 'boolean' };
        case ts.SyntaxKind.FunctionType:
            return { type: 'function' };
        case ts.SyntaxKind.TypeLiteral: {
            const shape = propsFromMembers(typeNode.members);
            return { type: 'object', shape };
        }
        default:
            return { type: 'unsupported' };
    }
}

function propsFromMembers(members) {
    const result = [];
    for (const member of members) {
        if (!ts.isPropertySignature(member) || !member.name || !ts.isIdentifier(member.name)) continue;
        const classified = classify(member.type);
        if (classified.type === 'unsupported') {
            warnings.push(`prop "${member.name.text}": unsupported type, using placeholder`);
        }
        result.push({
            name: member.name.text,
            type: classified.type,
            optional: !!member.questionToken,
            shape: classified.shape || null,
        });
    }
    return result;
}

const fn = findDefaultComponent(sourceFile);
if (!fn) {
    console.log(JSON.stringify({ props: [], warnings: ['no default-exported function component found'] }));
    process.exit(0);
}

if (fn.parameters.length === 0) {
    console.log(JSON.stringify({ props: [], warnings }));
    process.exit(0);
}

const propsParam = fn.parameters[0];
const members = resolveTypeMembers(propsParam.type);
const props = members ? propsFromMembers(members) : [];
if (!members) {
    warnings.push('could not resolve props type; rendering with no props');
}

console.log(JSON.stringify({ props, warnings }));
