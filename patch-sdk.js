// Normalizes the generated TypeScript SDK so it can be consumed from `file:`
// or published to npm. Pulumi's nodejs runtime reads the package version from
// ./package.json, so the SDK must compile to the package root (like the
// published @pulumi/* packages) instead of a bin/ subdirectory.
const fs = require("fs");
const path = require("path");

const sdkDir = path.join(__dirname, "sdk", "nodejs");
const pkgPath = path.join(sdkDir, "package.json");
const tsconfigPath = path.join(sdkDir, "tsconfig.json");

const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));
pkg.name = "@bambamboole/coolify";
pkg.main = "index.js";
pkg.typings = "index.d.ts";
pkg.scripts = pkg.scripts || {};
pkg.scripts.prepare = "tsc";
pkg.scripts.build = "tsc";
pkg.files = ["*.js", "*.d.ts", "*.ts", "config", "README.md", "LICENSE"];
pkg.publishConfig = { access: "public" };
fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + "\n");

const stripJsonComments = (s) => s.replace(/^\s*\/\/.*$/gm, "");

const tsconfig = JSON.parse(stripJsonComments(fs.readFileSync(tsconfigPath, "utf8")));
tsconfig.compilerOptions = tsconfig.compilerOptions || {};
tsconfig.compilerOptions.outDir = ".";
tsconfig.compilerOptions.declaration = true;
tsconfig.compilerOptions.declarationMap = true;
tsconfig.compilerOptions.sourceMap = true;
fs.writeFileSync(tsconfigPath, JSON.stringify(tsconfig, null, 4) + "\n");

console.log("patched", pkg.name, "outDir=.");