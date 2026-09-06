// Normalizes the generated TypeScript SDK's package.json: sets the npm package
// name and marks the package public. The SDK compiles into bin/ like the
// official @pulumi packages; `make build-sdk` copies this file there for
// publishing.
const fs = require("fs");
const path = require("path");

const pkgPath = path.join(__dirname, "sdk", "nodejs", "package.json");
const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));

pkg.name = "@bambamboole/coolify";
pkg.main = "index.js";
pkg.types = "index.d.ts";
pkg.publishConfig = { access: "public" };

fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + "\n");
console.log("patched", pkg.name);

// The generator uses a truthiness check for optional secrets, which drops
// value: "" instead of clearing a shared variable. Preserve explicit empties.
for (const scope of ["team", "project", "environment", "server"]) {
    const sourcePath = path.join(__dirname, "sdk", "nodejs", `${scope}SharedVariable.ts`);
    const source = fs.readFileSync(sourcePath, "utf8");
    const patched = source.replace(
        'args?.value ? pulumi.secret(args.value) : undefined',
        'args?.value !== undefined ? pulumi.secret(args.value) : undefined',
    );
    fs.writeFileSync(sourcePath, patched);
}
