package provider

import (
	"context"
	"encoding/base64"
	"fmt"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// Service manages a Coolify service (one-click template or docker compose)
// inside a project environment.
type Service struct{}

type ServiceArgs struct {
	// UUID of the Coolify project the service belongs to.
	ProjectUUID string `pulumi:"projectUuid"`
	// Name of the environment inside the project.
	EnvironmentName string `pulumi:"environmentName"`
	// UUID of the server hosting the service.
	ServerUUID string `pulumi:"serverUuid"`

	// Service name. Defaults to the Pulumi resource name.
	Name string `pulumi:"name,optional"`
	// Description of the service.
	Description string `pulumi:"description,optional"`
	// One-click service type, e.g. "plausible" or "gitea-with-mysql". Exactly one of type and dockerCompose must be set.
	Type string `pulumi:"type,optional"`
	// Docker compose file content for custom services. Exactly one of type and dockerCompose must be set.
	DockerCompose string `pulumi:"dockerCompose,optional"`
	// UUID of the destination when the server has several.
	DestinationUUID string `pulumi:"destinationUuid,optional"`
	// Start the service right after creating it.
	InstantDeploy bool `pulumi:"instantDeploy,optional"`
	// Connect the service to Coolify's predefined Docker network.
	ConnectToDockerNetwork bool `pulumi:"connectToDockerNetwork,optional"`
	// Environment variables managed by key, see Application.
	EnvironmentVariables map[string]string `pulumi:"environmentVariables,optional"`
	// Tags attached to the service in addition to the provider's default tags.
	Tags []string `pulumi:"tags,optional"`
}

type ServiceState struct {
	ServiceArgs
	// Tags the provider attached: the provider's default tags plus the declared ones.
	AppliedTags []string `pulumi:"appliedTags"`
	// UUID of the service in Coolify.
	UUID string `pulumi:"uuid"`
}

func (r *Service) Annotate(a infer.Annotator) {
	a.SetToken("index", "Service")
	a.Describe(&r, "A Coolify service from a one-click template or a docker compose file. An existing service with the same name in the environment is adopted on create.")
}

func (args *ServiceArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.ProjectUUID, "UUID of the Coolify project (the uuid output of a Project resource). Changing it moves the service to the new project in place.")
	a.Describe(&args.EnvironmentName, "Name of the environment inside the project. Changing it moves the service in place; the environment must already exist.")
	a.Describe(&args.ServerUUID, "UUID of the server hosting the service (the uuid output of a Server resource). Changing it replaces the service.")
	a.Describe(&args.Name, "Service name. Defaults to the Pulumi resource name. An existing service with this name in the environment is adopted.")
	a.Describe(&args.Description, "Description of the service.")
	a.Describe(&args.Type, `One-click service type, e.g. "plausible" or "gitea-with-mysql". Exactly one of type and dockerCompose must be set. Changing it replaces the service.`)
	a.Describe(&args.DockerCompose, "Docker compose file content for custom services. Exactly one of type and dockerCompose must be set. Coolify does not report the compose file back, so drift on this input is not detected.")
	a.Describe(&args.DestinationUUID, "UUID of the destination when the server has several. Only relevant on create.")
	a.Describe(&args.InstantDeploy, "Start the service right after creating it. Only relevant on create.")
	a.Describe(&args.ConnectToDockerNetwork, "Connect the service to Coolify's predefined Docker network.")
	a.Describe(&args.EnvironmentVariables, "Environment variables managed by key. Declared keys missing in Coolify are created as hidden values; existing keys are never patched and undeclared keys are left untouched.")
	a.Describe(&args.Tags, "Tags attached to the service in addition to the provider's default tags. Declared tags are attached, tags removed from the declaration are detached, tags added in the Coolify UI are left untouched.")
}

func (state *ServiceState) Annotate(a infer.Annotator) {
	a.Describe(&state.AppliedTags, "Tags the provider attached: the provider's default tags plus the declared ones.")
	a.Describe(&state.UUID, "UUID of the service in Coolify.")
}

func (Service) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[ServiceArgs], error) {
	args, failures, err := infer.DefaultCheck[ServiceArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[ServiceArgs]{}, err
	}
	if args.Name == "" {
		args.Name = req.Name
	}
	if (args.Type == "") == (args.DockerCompose == "") {
		failures = append(failures, p.CheckFailure{Property: "type", Reason: "exactly one of type and dockerCompose must be set"})
	}
	failures = append(failures, checkTags("tags", args.Tags)...)
	args.Tags = normalizeTags(args.Tags)
	return infer.CheckResponse[ServiceArgs]{Inputs: args, Failures: failures}, nil
}

func (Service) Create(ctx context.Context, req infer.CreateRequest[ServiceArgs]) (infer.CreateResponse[ServiceState], error) {
	if req.DryRun {
		return infer.CreateResponse[ServiceState]{Output: ServiceState{ServiceArgs: req.Inputs}}, nil
	}
	c := client(ctx)
	service, err := createService(ctx, c, req.Inputs)
	if err != nil {
		return infer.CreateResponse[ServiceState]{}, err
	}
	state := serviceState(req.Inputs, service)
	if state.AppliedTags, err = reconcileTags(ctx, c, serviceOwner(state.UUID), effectiveTags(ctx, req.Inputs.Tags), nil); err != nil {
		return infer.CreateResponse[ServiceState]{}, err
	}
	return infer.CreateResponse[ServiceState]{ID: state.UUID, Output: state}, nil
}

func (Service) Diff(ctx context.Context, req infer.DiffRequest[ServiceArgs, ServiceState]) (infer.DiffResponse, error) {
	// Project and environment changes move the service in place.
	diff := diffArgs(req.State.ServiceArgs, req.Inputs, "serverUuid", "type")
	// Only relevant on create.
	delete(diff, "instantDeploy")
	delete(diff, "destinationUuid")
	delete(diff, "environmentVariables")
	if environmentVariablesNeedUpdate(req.State.EnvironmentVariables, req.Inputs.EnvironmentVariables) {
		diff["environmentVariables"] = p.PropertyDiff{Kind: p.Update}
	}
	delete(diff, "tags")
	if tagsDiffer(effectiveTags(ctx, req.Inputs.Tags), req.State.AppliedTags) {
		diff["tags"] = p.PropertyDiff{Kind: p.Update}
	}
	return diffResponse(diff, req.State.Name == req.Inputs.Name), nil
}

func (Service) Update(ctx context.Context, req infer.UpdateRequest[ServiceArgs, ServiceState]) (infer.UpdateResponse[ServiceState], error) {
	if req.DryRun {
		return infer.UpdateResponse[ServiceState]{Output: ServiceState{ServiceArgs: req.Inputs, UUID: req.ID}}, nil
	}
	c := client(ctx)
	current, err := c.GetService(ctx, req.ID)
	if err != nil {
		return infer.UpdateResponse[ServiceState]{}, err
	}
	moved, err := ensurePlacement(ctx, c, servicePlacement(req.State.ServiceArgs), servicePlacement(req.Inputs),
		coolify.Deref(current.EnvironmentId), func(ctx context.Context, environmentUUID string) error {
			return c.MoveService(ctx, req.ID, environmentUUID)
		})
	if err != nil {
		return infer.UpdateResponse[ServiceState]{}, err
	}
	if moved {
		if current, err = c.GetService(ctx, req.ID); err != nil {
			return infer.UpdateResponse[ServiceState]{}, err
		}
	}
	service, err := applyService(ctx, c, current, req.Inputs, &req.State.DockerCompose)
	if err != nil {
		return infer.UpdateResponse[ServiceState]{}, err
	}
	state := serviceState(req.Inputs, service)
	if state.AppliedTags, err = reconcileTags(ctx, c, serviceOwner(req.ID), effectiveTags(ctx, req.Inputs.Tags), req.State.AppliedTags); err != nil {
		return infer.UpdateResponse[ServiceState]{}, err
	}
	return infer.UpdateResponse[ServiceState]{Output: state}, nil
}

func (Service) Read(ctx context.Context, req infer.ReadRequest[ServiceArgs, ServiceState]) (infer.ReadResponse[ServiceArgs, ServiceState], error) {
	c := client(ctx)
	service, err := c.GetService(ctx, req.ID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[ServiceArgs, ServiceState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[ServiceArgs, ServiceState]{}, err
	}
	inputs := serviceInputs(req.Inputs, service)
	if len(req.Inputs.EnvironmentVariables) > 0 {
		existing, err := c.ListServiceEnvVars(ctx, req.ID)
		if err != nil {
			return infer.ReadResponse[ServiceArgs, ServiceState]{}, err
		}
		inputs.EnvironmentVariables = declaredEnvironmentVariables(req.Inputs.EnvironmentVariables, existing)
	}
	tags, applied, err := readTags(ctx, c, serviceOwner(req.ID), req.Inputs.Tags, req.State.AppliedTags)
	if err != nil {
		return infer.ReadResponse[ServiceArgs, ServiceState]{}, err
	}
	inputs.Tags = tags
	state := serviceState(inputs, service)
	state.AppliedTags = applied
	return infer.ReadResponse[ServiceArgs, ServiceState]{ID: req.ID, Inputs: inputs, State: state}, nil
}

func (Service) Delete(ctx context.Context, req infer.DeleteRequest[ServiceState]) (infer.DeleteResponse, error) {
	if err := client(ctx).DeleteService(ctx, req.ID); err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func serviceOwner(uuid string) coolify.Owner {
	return coolify.Owner{Kind: coolify.OwnerService, UUID: uuid}
}

func servicePlacement(args ServiceArgs) placement {
	return placement{ProjectUUID: args.ProjectUUID, EnvironmentName: args.EnvironmentName}
}

// createService adopts the service with the same name in the environment or
// creates it, then reconciles its settings with the inputs.
func createService(ctx context.Context, c *coolify.Client, inputs ServiceArgs) (api.Service, error) {
	environment, err := resolveEnvironment(ctx, c, inputs.ProjectUUID, inputs.EnvironmentName)
	if err != nil {
		return api.Service{}, err
	}
	services, err := c.ListServices(ctx)
	if err != nil {
		return api.Service{}, err
	}
	for _, candidate := range services {
		if coolify.Deref(candidate.Name) != inputs.Name || coolify.Deref(candidate.EnvironmentId) != environment.ID {
			continue
		}
		if inputs.Type != "" && coolify.Deref(candidate.ServiceType) != inputs.Type {
			return api.Service{}, fmt.Errorf("coolify service %q already exists in environment %q with type %q, expected %q",
				inputs.Name, inputs.EnvironmentName, coolify.Deref(candidate.ServiceType), inputs.Type)
		}
		// The compose file is unknown on adoption, so it is always applied.
		return applyService(ctx, c, candidate, inputs, nil)
	}
	uuid, err := c.CreateService(ctx, api.CreateServiceJSONRequestBody{
		ProjectUuid:      inputs.ProjectUUID,
		EnvironmentName:  environment.Name,
		EnvironmentUuid:  environment.UUID,
		ServerUuid:       inputs.ServerUUID,
		Name:             &inputs.Name,
		Description:      coolify.PtrIfNonZero(inputs.Description),
		Type:             coolify.PtrIfNonZero(inputs.Type),
		DockerComposeRaw: coolify.PtrIfNonZero(encodeCompose(inputs.DockerCompose)),
		DestinationUuid:  coolify.PtrIfNonZero(inputs.DestinationUUID),
		InstantDeploy:    coolify.PtrIfNonZero(inputs.InstantDeploy),
	})
	if err != nil {
		return api.Service{}, err
	}
	created, err := c.GetService(ctx, uuid)
	if err != nil {
		return api.Service{}, err
	}
	// The compose file went into the create request; only the remaining
	// settings and the environment variables are applied afterwards.
	return applyService(ctx, c, created, inputs, &inputs.DockerCompose)
}

// applyService patches the fields of current that differ from the inputs and
// ensures the declared environment variables. Coolify hides the compose file,
// so it is sent whenever it is unknown (previousCompose is nil) or differs
// from the previous inputs.
func applyService(ctx context.Context, c *coolify.Client, current api.Service, inputs ServiceArgs, previousCompose *string) (api.Service, error) {
	uuid := coolify.Deref(current.Uuid)
	var body api.UpdateServiceByUuidJSONRequestBody
	var patch patch
	patch.str(&body.Name, inputs.Name, coolify.Deref(current.Name))
	patch.text(&body.Description, inputs.Description, coolify.Deref(current.Description))
	patch.boolean(&body.ConnectToDockerNetwork, inputs.ConnectToDockerNetwork, coolify.Deref(current.ConnectToDockerNetwork))
	if inputs.DockerCompose != "" && (previousCompose == nil || *previousCompose != inputs.DockerCompose) {
		body.DockerComposeRaw = coolify.Ptr(encodeCompose(inputs.DockerCompose))
		patch.changed = true
	}
	if patch.changed {
		if err := c.UpdateService(ctx, uuid, body); err != nil {
			return api.Service{}, err
		}
	}
	if err := ensureEnvironmentVariables(ctx, serviceEnvVars(c, uuid), inputs.EnvironmentVariables); err != nil {
		return api.Service{}, err
	}
	if !patch.changed {
		return current, nil
	}
	return c.GetService(ctx, uuid)
}

// serviceEnvVars adapts the service environment variable endpoints.
func serviceEnvVars(c *coolify.Client, uuid string) envVars {
	return envVars{
		list: func(ctx context.Context) ([]api.EnvironmentVariable, error) {
			return c.ListServiceEnvVars(ctx, uuid)
		},
		create: func(ctx context.Context, key, value string) error {
			_, err := c.CreateServiceEnvVar(ctx, uuid, api.CreateEnvByServiceUuidJSONRequestBody{
				Key:         coolify.Ptr(key),
				Value:       coolify.Ptr(value),
				IsLiteral:   coolify.Ptr(true),
				IsPreview:   coolify.Ptr(false),
				IsShownOnce: coolify.Ptr(true),
			})
			return err
		},
	}
}

// encodeCompose base64-encodes the compose file as the API requires; empty
// input stays empty so it is omitted from request bodies.
func encodeCompose(compose string) string {
	if compose == "" {
		return ""
	}
	return base64.StdEncoding.EncodeToString([]byte(compose))
}

// serviceInputs derives the inputs from the service Coolify reports, keeping
// the identity, the compose file and unmanaged optional inputs.
func serviceInputs(previous ServiceArgs, service api.Service) ServiceArgs {
	inputs := previous
	inputs.Name = coolify.Deref(service.Name)
	inputs.Description = coolify.Deref(service.Description)
	inputs.Type = ifSet(previous.Type, coolify.Deref(service.ServiceType))
	inputs.ConnectToDockerNetwork = coolify.Deref(service.ConnectToDockerNetwork)
	return inputs
}

func serviceState(inputs ServiceArgs, service api.Service) ServiceState {
	return ServiceState{ServiceArgs: inputs, UUID: coolify.Deref(service.Uuid)}
}
