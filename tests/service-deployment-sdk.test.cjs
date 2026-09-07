const assert = require("node:assert/strict");
const { test } = require("node:test");
const pulumi = require("../sdk/nodejs/node_modules/@pulumi/pulumi");
const coolify = require("../sdk/nodejs/bin");

test("service deployment SDK resolves service output and trigger inputs", async () => {
    const registrations = new Map();
    await pulumi.runtime.setMocks({
        newResource(args) {
            registrations.set(args.name, args);
            return { id: `${args.name}-id`, state: { ...args.inputs, uuid: `${args.name}-uuid`, status: "queued" } };
        },
        call(args) { return args.inputs; },
    }, "test", "test", false);
    const service = new coolify.Service("work", {
        projectUuid: "project", environmentName: "production", serverUuid: "server", type: "example",
    });
    const deployment = new coolify.ServiceDeployment("deploy-work", {
        service: service.uuid,
        triggers: [pulumi.output("image-v2"), "config-v3"],
    });
    await deployment.urn.promise();
    assert.equal(registrations.get("deploy-work").type, "coolify:index:ServiceDeployment");
    assert.deepEqual(registrations.get("deploy-work").inputs, {
        service: "work-uuid", triggers: ["image-v2", "config-v3"],
    });
    assert.equal(await deployment.status.promise(), "queued");
});
