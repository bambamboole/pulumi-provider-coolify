const assert = require("node:assert/strict");
const { test } = require("node:test");
const pulumi = require("../sdk/nodejs/node_modules/@pulumi/pulumi");
const coolify = require("../sdk/nodejs/bin");

test("service SDK preserves domain inputs, clears and omission", async () => {
    const registrations = new Map();
    await pulumi.runtime.setMocks({
        newResource(args) {
            registrations.set(args.name, args.inputs);
            return { id: `${args.name}-id`, state: args.inputs };
        },
        call(args) { return args.inputs; },
    }, "test", "test", false);
    for (const [name, domains] of [
        ["assigned", { affine: pulumi.output("https://work.example.com:3010") }],
        ["cleared", { affine: "" }],
        ["unmanaged", undefined],
    ]) {
        const service = new coolify.Service(name, {
            projectUuid: "project", environmentName: "production", serverUuid: "server",
            dockerCompose: "services:\n  affine:\n    image: example\n", domains,
        });
        await service.urn.promise();
        assert.deepEqual(registrations.get(name).domains,
            name === "assigned" ? { affine: "https://work.example.com:3010" } : domains);
    }
});
