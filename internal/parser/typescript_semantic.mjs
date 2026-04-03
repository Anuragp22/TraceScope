import fs from "node:fs";
import path from "node:path";
import { fileURLToPath, pathToFileURL } from "node:url";

const scriptDir = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(scriptDir, "..", "..");
const localTypeScript = path.join(repoRoot, "web", "node_modules", "typescript", "lib", "typescript.js");
const tsModule = await import(pathToFileURL(localTypeScript).href);
const ts = tsModule.default ?? tsModule;

const input = JSON.parse(fs.readFileSync(0, "utf8") || '{"files":[]}');
const files = Array.isArray(input.files) ? input.files : [];

const program = ts.createProgram(files, {
  allowJs: false,
  allowSyntheticDefaultImports: true,
  esModuleInterop: true,
  jsx: ts.JsxEmit.Preserve,
  module: ts.ModuleKind.ESNext,
  moduleResolution: ts.ModuleResolutionKind.NodeJs,
  noEmit: true,
  skipLibCheck: true,
  strict: false,
  target: ts.ScriptTarget.ESNext,
});

const checker = program.getTypeChecker();
const wanted = new Set(files.map((file) => normalizePath(file)));
const result = [];

for (const sourceFile of program.getSourceFiles()) {
  const filePath = normalizePath(sourceFile.fileName);
  if (!wanted.has(filePath)) {
    continue;
  }

  const calls = [];
  visit(sourceFile, sourceFile, calls);
  result.push({ filePath, calls });
}

process.stdout.write(JSON.stringify(result));

function visit(sourceFile, node, calls) {
  if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression)) {
    const receiverExpr = node.expression.expression;
    const receiver = receiverExpr.getText(sourceFile);
    const receiverInfo = inferReceiverType(receiverExpr);
    if (receiverInfo.name) {
      const pos = sourceFile.getLineAndCharacterOfPosition(node.getStart(sourceFile));
      calls.push({
        line: pos.line + 1,
        name: node.expression.name.text,
        receiver,
        receiverType: receiverInfo.name,
        receiverPackage: receiverInfo.filePath,
      });
    }
  }

  ts.forEachChild(node, (child) => visit(sourceFile, child, calls));
}

function inferReceiverType(node) {
  const type = checker.getTypeAtLocation(node);
  return extractTypeName(type);
}

function extractTypeName(type) {
  if (!type) {
    return { name: "", filePath: "" };
  }

  if (typeof type.isUnion === "function" && type.isUnion()) {
    const members = type.types
      .map((member) => extractTypeName(member))
      .filter((member) => member.name);
    const keys = new Set(members.map((member) => `${member.filePath}|${member.name}`));
    return keys.size === 1 ? members[0] : { name: "", filePath: "" };
  }

  const symbol = type.aliasSymbol || type.symbol;
  if (symbol?.name && symbol.name !== "__type") {
    return {
      name: symbol.name,
      filePath: declarationFilePath(symbol),
    };
  }

  const text = checker.typeToString(type).replace(/\[\]$/, "");
  return /^[A-Za-z_$][\w$]*$/.test(text)
    ? { name: text, filePath: "" }
    : { name: "", filePath: "" };
}

function declarationFilePath(symbol) {
  const declarations = symbol?.getDeclarations?.() || symbol?.declarations || [];
  if (!declarations.length) {
    return "";
  }
  return normalizePath(declarations[0].getSourceFile().fileName);
}

function normalizePath(filePath) {
  return path.resolve(filePath).replaceAll("\\", "/");
}
