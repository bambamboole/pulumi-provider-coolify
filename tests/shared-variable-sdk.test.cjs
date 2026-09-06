const assert = require("node:assert/strict");
const { test } = require("node:test");
const pulumi = require("../sdk/nodejs/node_modules/@pulumi/pulumi");
const coolify = require("../sdk/nodejs/bin");

test("shared variable SDK preserves empty, secret and omitted values", async () => {
    const registrations = new Map();
    await pulumi.runtime.setMocks({
        newResource(args) {
            registrations.set(args.name, args.inputs);
            return { id: `${args.name}-id`, state: args.inputs };
        },
        call(args) { return args.inputs; },
    }, "test", "test", false);

    const scopes = [
        [coolify.TeamSharedVariable, {}],
        [coolify.ProjectSharedVariable, { projectUuid: "project" }],
        [coolify.EnvironmentSharedVariable, { projectUuid: "project", environmentName: "production" }],
        [coolify.ServerSharedVariable, { serverUuid: "server" }],
    ];
    for (const [Resource, scope] of scopes) {
        for (const [label, value] of [["empty", ""], ["secret", pulumi.secret("rotated")], ["omitted", undefined]]) {
            const name = `${Resource.name}-${label}`;
            const variable = new Resource(name, { ...scope, key: "TOKEN", value });
            await variable.urn.promise();
            const sent = registrations.get(name);
            assert.ok(sent, `${name} was not registered`);
            if (label === "omitted") {
                assert.equal(sent.value, undefined);
            } else {
                // Pulumi serializes secrets as a signature plus their value.
                assert.equal(sent.value.value, label === "empty" ? "" : "rotated");
                assert.equal(await pulumi.isSecret(variable.value), true);
            }
        }
    }
});
