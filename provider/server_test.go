package provider

import (
	"context"
	"testing"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

func serverArgs(privateKeyUUID string) ServerArgs {
	return ServerArgs{
		Name:           "app-1",
		IP:             "203.0.113.10",
		Port:           22,
		User:           "root",
		PrivateKeyUUID: privateKeyUUID,
	}
}

func TestServerReadResolvesPrivateKeyUUID(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := withClient(context.Background(), c)
	keyUUID := fake.addPrivateKey("deploy")

	server, err := createServer(ctx, c, serverArgs(keyUUID))
	if err != nil {
		t.Fatalf("createServer: %v", err)
	}
	uuid := coolify.Deref(server.Uuid)

	read, err := (Server{}).Read(ctx, infer.ReadRequest[ServerArgs, ServerState]{ID: uuid, Inputs: ServerArgs{PrivateKeyUUID: "stale"}})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if read.Inputs.PrivateKeyUUID != keyUUID || read.State.PrivateKeyUUID != keyUUID {
		t.Fatalf("Read must resolve the private key uuid, got %+v", read.Inputs)
	}
	if read.Inputs.Name != "app-1" || read.Inputs.IP != "203.0.113.10" || read.State.UUID != uuid {
		t.Fatalf("unexpected read result: %+v", read)
	}

	// Without a matching key the previous input is kept.
	fake.mu.Lock()
	fake.servers[uuid]["private_key_id"] = 999
	fake.mu.Unlock()
	read, err = (Server{}).Read(ctx, infer.ReadRequest[ServerArgs, ServerState]{ID: uuid, Inputs: ServerArgs{PrivateKeyUUID: "previous"}})
	if err != nil {
		t.Fatalf("Read with unknown key: %v", err)
	}
	if read.Inputs.PrivateKeyUUID != "previous" {
		t.Fatalf("Read must keep the previous key when none matches, got %+v", read.Inputs)
	}

	// A deleted server reads as missing.
	if _, err := (Server{}).Delete(ctx, infer.DeleteRequest[ServerState]{ID: uuid}); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	missing, err := (Server{}).Read(ctx, infer.ReadRequest[ServerArgs, ServerState]{ID: uuid})
	if err != nil || missing.ID != "" {
		t.Fatalf("deleted server must read as missing, got %+v, %v", missing, err)
	}
}

func TestCreateServerAdoptsWithoutRepatchingUnchangedKey(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	keyUUID := fake.addPrivateKey("deploy")
	otherKeyUUID := fake.addPrivateKey("other")

	server, err := createServer(ctx, c, serverArgs(keyUUID))
	if err != nil {
		t.Fatalf("createServer: %v", err)
	}
	uuid := coolify.Deref(server.Uuid)
	path := "/api/v1/servers/" + uuid

	adopted, err := createServer(ctx, c, serverArgs(keyUUID))
	if err != nil {
		t.Fatalf("adopt: %v", err)
	}
	if coolify.Deref(adopted.Uuid) != uuid {
		t.Fatalf("server was not adopted: %+v", adopted)
	}
	if n := fake.countRequests("PATCH", path); n != 0 {
		t.Fatalf("adopt with the same key must not patch, got %v", fake.requests)
	}

	adopted, err = createServer(ctx, c, serverArgs(otherKeyUUID))
	if err != nil {
		t.Fatalf("adopt with other key: %v", err)
	}
	if coolify.Deref(adopted.Uuid) != uuid {
		t.Fatalf("server was not adopted: %+v", adopted)
	}
	if n := fake.countRequests("PATCH", path); n != 1 {
		t.Fatalf("adopt with a changed key must patch once, got %v", fake.requests)
	}
	details, err := c.GetServerDetails(ctx, uuid)
	if err != nil {
		t.Fatalf("GetServerDetails: %v", err)
	}
	if got, err := c.PrivateKeyUUIDByID(ctx, details.PrivateKeyID); err != nil || got != otherKeyUUID {
		t.Fatalf("patched key must be the other key, got %q, %v", got, err)
	}
	if fake.countRequests("POST", "/api/v1/servers") != 1 {
		t.Fatalf("server was recreated: %v", fake.requests)
	}
}
