package provider

import (
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// Server manages a Coolify server and the SSH connection to it.
type Server struct{}

type ServerArgs struct {
	// Name of the server. An existing server with this name is adopted.
	Name string `pulumi:"name"`
	// Description of the server.
	Description string `pulumi:"description,optional"`
	// IP address or hostname of the server.
	IP string `pulumi:"ip"`
	// SSH port. Defaults to 22.
	Port int `pulumi:"port,optional"`
	// SSH user. Defaults to root.
	User string `pulumi:"user,optional"`
	// UUID of the Coolify private key used to connect.
	PrivateKeyUUID string `pulumi:"privateKeyUuid"`
}

type ServerState struct {
	ServerArgs
	// UUID of the server in Coolify.
	UUID string `pulumi:"uuid"`
}

func (r *Server) Annotate(a infer.Annotator) {
	a.SetToken("index", "Server")
	a.Describe(&r, "A Coolify server connected over SSH with a Coolify private key. An existing server with the same name is adopted on create.")
}

func (args *ServerArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "Name of the server. An existing server with this name is adopted.")
	a.Describe(&args.Description, "Description of the server.")
	a.Describe(&args.IP, "IP address or hostname of the server.")
	a.Describe(&args.Port, "SSH port.")
	a.Describe(&args.User, "SSH user.")
	a.Describe(&args.PrivateKeyUUID, "UUID of the Coolify private key used to connect (the uuid output of a PrivateKey resource). Resolved from Coolify on read, so an adopted server is not re-patched when the key is unchanged.")
	a.SetDefault(&args.Port, 22)
	a.SetDefault(&args.User, "root")
}

func (state *ServerState) Annotate(a infer.Annotator) {
	a.Describe(&state.UUID, "UUID of the server in Coolify.")
}

func (Server) Create(ctx context.Context, req infer.CreateRequest[ServerArgs]) (infer.CreateResponse[ServerState], error) {
	if req.DryRun {
		return infer.CreateResponse[ServerState]{Output: ServerState{ServerArgs: req.Inputs}}, nil
	}
	server, err := createServer(ctx, client(ctx), req.Inputs)
	if err != nil {
		return infer.CreateResponse[ServerState]{}, err
	}
	return infer.CreateResponse[ServerState]{ID: coolify.Deref(server.Uuid), Output: serverState(req.Inputs, server)}, nil
}

func (Server) Diff(ctx context.Context, req infer.DiffRequest[ServerArgs, ServerState]) (infer.DiffResponse, error) {
	return diffResponse(diffArgs(req.State.ServerArgs, req.Inputs), true), nil
}

func (Server) Update(ctx context.Context, req infer.UpdateRequest[ServerArgs, ServerState]) (infer.UpdateResponse[ServerState], error) {
	if req.DryRun {
		return infer.UpdateResponse[ServerState]{Output: ServerState{ServerArgs: req.Inputs, UUID: req.ID}}, nil
	}
	c := client(ctx)
	current, err := c.GetServerDetails(ctx, req.ID)
	if err != nil {
		return infer.UpdateResponse[ServerState]{}, err
	}
	// Compare the key against Coolify, not only the recorded state, so a key
	// changed outside Pulumi is restored on the next update.
	keyUUID, err := resolvePrivateKeyUUID(ctx, c, current.PrivateKeyID, req.State.PrivateKeyUUID)
	if err != nil {
		return infer.UpdateResponse[ServerState]{}, err
	}
	server, err := applyServer(ctx, c, current.Server, keyUUID, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[ServerState]{}, err
	}
	return infer.UpdateResponse[ServerState]{Output: serverState(req.Inputs, server)}, nil
}

func (Server) Read(ctx context.Context, req infer.ReadRequest[ServerArgs, ServerState]) (infer.ReadResponse[ServerArgs, ServerState], error) {
	c := client(ctx)
	details, err := c.GetServerDetails(ctx, req.ID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[ServerArgs, ServerState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[ServerArgs, ServerState]{}, err
	}
	keyUUID, err := resolvePrivateKeyUUID(ctx, c, details.PrivateKeyID, req.Inputs.PrivateKeyUUID)
	if err != nil {
		return infer.ReadResponse[ServerArgs, ServerState]{}, err
	}
	server := details.Server
	inputs := req.Inputs
	inputs.Name = coolify.Deref(server.Name)
	inputs.Description = coolify.Deref(server.Description)
	inputs.IP = coolify.Deref(server.Ip)
	inputs.Port = coolify.Deref(server.Port)
	inputs.User = coolify.Deref(server.User)
	inputs.PrivateKeyUUID = keyUUID
	return infer.ReadResponse[ServerArgs, ServerState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  serverState(inputs, server),
	}, nil
}

func (Server) Delete(ctx context.Context, req infer.DeleteRequest[ServerState]) (infer.DeleteResponse, error) {
	if err := client(ctx).DeleteServer(ctx, req.ID); err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

// createServer adopts the server with the given name or creates it.
func createServer(ctx context.Context, c *coolify.Client, inputs ServerArgs) (api.Server, error) {
	servers, err := c.ListServers(ctx)
	if err != nil {
		return api.Server{}, err
	}
	for _, server := range servers {
		if coolify.Deref(server.Name) != inputs.Name {
			continue
		}
		// The list omits the private key; the single-server endpoint reports
		// its ID so an unchanged key is not re-applied.
		details, err := c.GetServerDetails(ctx, coolify.Deref(server.Uuid))
		if err != nil {
			return api.Server{}, err
		}
		keyUUID, err := resolvePrivateKeyUUID(ctx, c, details.PrivateKeyID, "")
		if err != nil {
			return api.Server{}, err
		}
		return applyServer(ctx, c, details.Server, keyUUID, inputs)
	}
	uuid, err := c.CreateServer(ctx, api.CreateServerJSONRequestBody{
		Name:            &inputs.Name,
		Description:     &inputs.Description,
		Ip:              &inputs.IP,
		Port:            &inputs.Port,
		User:            &inputs.User,
		PrivateKeyUuid:  &inputs.PrivateKeyUUID,
		InstantValidate: coolify.Ptr(true),
	})
	if err != nil {
		return api.Server{}, err
	}
	return c.GetServer(ctx, uuid)
}

// applyServer patches the fields of current that differ from the inputs. The
// private key is compared against currentKeyUUID because api.Server omits it.
func applyServer(ctx context.Context, c *coolify.Client, current api.Server, currentKeyUUID string, inputs ServerArgs) (api.Server, error) {
	var body api.UpdateServerByUuidJSONRequestBody
	var patch patch
	patch.str(&body.Name, inputs.Name, coolify.Deref(current.Name))
	patch.text(&body.Description, inputs.Description, coolify.Deref(current.Description))
	patch.str(&body.Ip, inputs.IP, coolify.Deref(current.Ip))
	patch.integer(&body.Port, inputs.Port, current.Port)
	patch.str(&body.User, inputs.User, coolify.Deref(current.User))
	patch.str(&body.PrivateKeyUuid, inputs.PrivateKeyUUID, currentKeyUUID)
	if !patch.changed {
		return current, nil
	}
	uuid := coolify.Deref(current.Uuid)
	if err := c.UpdateServer(ctx, uuid, body); err != nil {
		return api.Server{}, err
	}
	return c.GetServer(ctx, uuid)
}

// resolvePrivateKeyUUID maps the private key ID Coolify reports to the key's
// UUID. It falls back to previous when the ID is missing or no key matches,
// e.g. because the token cannot list the key.
func resolvePrivateKeyUUID(ctx context.Context, c *coolify.Client, keyID int, previous string) (string, error) {
	uuid, err := c.PrivateKeyUUIDByID(ctx, keyID)
	if err != nil {
		return "", err
	}
	if uuid == "" {
		return previous, nil
	}
	return uuid, nil
}

func serverState(inputs ServerArgs, server api.Server) ServerState {
	return ServerState{ServerArgs: inputs, UUID: coolify.Deref(server.Uuid)}
}
