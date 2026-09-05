package provider

import (
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// Server manages a Coolify server and the SSH connection to it.
type Server struct{}

type ServerArgs struct {
	// Name of the server.
	Name string `pulumi:"name"`
	// Description of the server.
	Description string `pulumi:"description,optional"`
	// IP address of the server.
	IP string `pulumi:"ip"`
	// SSH port of the server.
	Port int `pulumi:"port,optional"`
	// SSH user of the server.
	User string `pulumi:"user,optional"`
	// UUID of the Coolify private key used to connect.
	PrivateKeyUUID string `pulumi:"privateKeyUuid"`
}

type ServerState struct {
	// UUID of the server in Coolify.
	UUID string `pulumi:"uuid"`
	// Name of the server.
	Name string `pulumi:"name"`
	// Description of the server.
	Description string `pulumi:"description"`
	// IP address of the server.
	IP string `pulumi:"ip"`
	// SSH port of the server.
	Port int `pulumi:"port"`
	// SSH user of the server.
	User string `pulumi:"user"`
	// UUID of the Coolify private key used to connect.
	PrivateKeyUUID string `pulumi:"privateKeyUuid"`
}

func (r *Server) Annotate(a infer.Annotator) {
	a.SetToken("index", "Server")
	a.Describe(&r, "A Coolify server connected over SSH with a Coolify private key.")
}

func (args *ServerArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.PrivateKeyUUID, "UUID of the Coolify private key used to connect. Usually the id output of a PrivateKey resource.")
}

func (Server) Create(ctx context.Context, req infer.CreateRequest[ServerArgs]) (infer.CreateResponse[ServerState], error) {
	if req.DryRun {
		return infer.CreateResponse[ServerState]{ID: "pending", Output: serverState("pending", req.Inputs)}, nil
	}
	c := client(ctx)
	state, err := syncServer(ctx, c, req.Inputs)
	if err != nil {
		return infer.CreateResponse[ServerState]{}, err
	}
	return infer.CreateResponse[ServerState]{ID: state.UUID, Output: state}, nil
}

func (Server) Update(ctx context.Context, req infer.UpdateRequest[ServerArgs, ServerState]) (infer.UpdateResponse[ServerState], error) {
	if req.DryRun {
		return infer.UpdateResponse[ServerState]{Output: serverState(req.State.UUID, req.Inputs)}, nil
	}
	c := client(ctx)
	state, err := syncServer(ctx, c, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[ServerState]{}, err
	}
	return infer.UpdateResponse[ServerState]{Output: state}, nil
}

func (Server) Read(ctx context.Context, req infer.ReadRequest[ServerArgs, ServerState]) (infer.ReadResponse[ServerArgs, ServerState], error) {
	c := client(ctx)
	server, err := c.GetServer(ctx, req.ID)
	if err != nil {
		return infer.ReadResponse[ServerArgs, ServerState]{}, err
	}
	inputs := req.Inputs
	if inputs.Name == "" {
		inputs.Name = server.Name
		inputs.Description = desc(server.Description)
		inputs.IP = server.IP
		inputs.Port = server.Port
		inputs.User = server.User
		inputs.PrivateKeyUUID = req.State.PrivateKeyUUID
	}
	return infer.ReadResponse[ServerArgs, ServerState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  readServerState(server, req.State.PrivateKeyUUID),
	}, nil
}

func (Server) Delete(ctx context.Context, req infer.DeleteRequest[ServerState]) (infer.DeleteResponse, error) {
	c := client(ctx)
	if err := c.DeleteServer(ctx, req.State.UUID); err != nil && !NotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func syncServer(ctx context.Context, c *Client, inputs ServerArgs) (ServerState, error) {

	servers, err := c.ListServers(ctx)
	if err != nil {
		return ServerState{}, err
	}
	var existing *CoolifyServer
	for i := range servers {
		if servers[i].Name == inputs.Name {
			existing = &servers[i]
			break
		}
	}

	if existing == nil {
		uuid, err := c.CreateServer(ctx, CreateServerInput{
			Name:           inputs.Name,
			Description:    inputs.Description,
			IP:             inputs.IP,
			Port:           inputs.Port,
			User:           inputs.User,
			PrivateKeyUUID: inputs.PrivateKeyUUID,
		})
		if err != nil {
			return ServerState{}, err
		}
		server, err := c.GetServer(ctx, uuid)
		if err != nil {
			return ServerState{}, err
		}
		return readServerState(server, inputs.PrivateKeyUUID), nil
	}

	changes := map[string]any{}
	if existing.Name != inputs.Name {
		changes["name"] = inputs.Name
	}
	if desc(existing.Description) != inputs.Description {
		changes["description"] = inputs.Description
	}
	if existing.IP != inputs.IP {
		changes["ip"] = inputs.IP
	}
	if existing.Port != inputs.Port {
		changes["port"] = inputs.Port
	}
	if existing.User != inputs.User {
		changes["user"] = inputs.User
	}
	if len(changes) > 0 {
		if err := c.UpdateServer(ctx, existing.UUID, changes); err != nil {
			return ServerState{}, err
		}
		server, err := c.GetServer(ctx, existing.UUID)
		if err != nil {
			return ServerState{}, err
		}
		return readServerState(server, inputs.PrivateKeyUUID), nil
	}

	return readServerState(*existing, inputs.PrivateKeyUUID), nil
}

// readServerState normalizes a CoolifyServer into state, using the provider's
// desired private key UUID because the server API response does not include it.
func readServerState(server CoolifyServer, privateKeyUUID string) ServerState {
	return ServerState{
		UUID:           server.UUID,
		Name:           server.Name,
		Description:    desc(server.Description),
		IP:             server.IP,
		Port:           server.Port,
		User:           server.User,
		PrivateKeyUUID: privateKeyUUID,
	}
}

func serverState(uuid string, inputs ServerArgs) ServerState {
	return ServerState{
		UUID:           uuid,
		Name:           inputs.Name,
		Description:    inputs.Description,
		IP:             inputs.IP,
		Port:           inputs.Port,
		User:           inputs.User,
		PrivateKeyUUID: inputs.PrivateKeyUUID,
	}
}
