package provider

import (
	"context"
	"fmt"
	"slices"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

// ServiceDeployment queues recreation of a service's containers when its
// declared triggers change. Completion means request acceptance, not health.
type ServiceDeployment struct{}

type ServiceDeploymentArgs struct {
	Service  string   `pulumi:"service"`
	Triggers []string `pulumi:"triggers,optional"`
}

type ServiceDeploymentState struct {
	ServiceDeploymentArgs
	Status string `pulumi:"status"`
}

func (r *ServiceDeployment) Annotate(a infer.Annotator) {
	a.SetToken("index", "ServiceDeployment")
	a.Describe(&r, "Queues a Coolify service restart on create and when triggers change. Coolify recreates the configured containers asynchronously and may briefly interrupt service. Completion acknowledges the request only; it does not verify deployment completion or health. Deleting this resource leaves the service running.")
}

func (args *ServiceDeploymentArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Service, "UUID of the Coolify service to redeploy (the uuid output of a Service resource). Changing it replaces this request resource. Declare at most one ServiceDeployment for each service.")
	a.Describe(&args.Triggers, "Values that queue a new restart when changed, such as an image version or configuration digest. Include every configuration change that should redeploy; updating a Service alone does not change these triggers.")
}

func (state *ServiceDeploymentState) Annotate(a infer.Annotator) {
	a.Describe(&state.Status, "The last request acknowledgment: queued after Coolify accepts a restart request. This is not the current deployment or health status and is not polled. Empty for imported resources whose request history is unknown.")
}

func (ServiceDeployment) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[ServiceDeploymentArgs], error) {
	args, failures, err := infer.DefaultCheck[ServiceDeploymentArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[ServiceDeploymentArgs]{}, err
	}
	if strings.TrimSpace(args.Service) == "" && !req.NewInputs.Get("service").IsComputed() && len(failures) == 0 {
		failures = append(failures, p.CheckFailure{Property: "service", Reason: "service UUID must not be empty"})
	}
	return infer.CheckResponse[ServiceDeploymentArgs]{Inputs: args, Failures: failures}, nil
}

func (ServiceDeployment) Create(ctx context.Context, req infer.CreateRequest[ServiceDeploymentArgs]) (infer.CreateResponse[ServiceDeploymentState], error) {
	state := ServiceDeploymentState{ServiceDeploymentArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[ServiceDeploymentState]{Output: state}, nil
	}
	if err := client(ctx).RestartService(ctx, req.Inputs.Service); err != nil {
		return infer.CreateResponse[ServiceDeploymentState]{}, err
	}
	state.Status = "queued"
	return infer.CreateResponse[ServiceDeploymentState]{ID: req.Inputs.Service, Output: state}, nil
}

func (ServiceDeployment) Diff(_ context.Context, req infer.DiffRequest[ServiceDeploymentArgs, ServiceDeploymentState]) (infer.DiffResponse, error) {
	diff := map[string]p.PropertyDiff{}
	if req.State.Service != req.Inputs.Service {
		diff["service"] = p.PropertyDiff{Kind: p.UpdateReplace}
	}
	if !slices.Equal(req.State.Triggers, req.Inputs.Triggers) {
		diff["triggers"] = p.PropertyDiff{Kind: p.Update}
	}
	return diffResponse(diff, false), nil
}

func (ServiceDeployment) Update(ctx context.Context, req infer.UpdateRequest[ServiceDeploymentArgs, ServiceDeploymentState]) (infer.UpdateResponse[ServiceDeploymentState], error) {
	state := req.State
	state.ServiceDeploymentArgs = req.Inputs
	if req.DryRun {
		return infer.UpdateResponse[ServiceDeploymentState]{Output: state}, nil
	}
	if req.ID != req.Inputs.Service {
		return infer.UpdateResponse[ServiceDeploymentState]{}, fmt.Errorf("changing service requires replacement of ServiceDeployment")
	}
	if slices.Equal(req.State.Triggers, req.Inputs.Triggers) {
		return infer.UpdateResponse[ServiceDeploymentState]{Output: state}, nil
	}
	if err := client(ctx).RestartService(ctx, req.Inputs.Service); err != nil {
		return infer.UpdateResponse[ServiceDeploymentState]{}, err
	}
	state.Status = "queued"
	return infer.UpdateResponse[ServiceDeploymentState]{Output: state}, nil
}

// Read checks only the owning service's existence. Runtime health and missing
// request history must never cause this action to replay during refresh.
func (ServiceDeployment) Read(ctx context.Context, req infer.ReadRequest[ServiceDeploymentArgs, ServiceDeploymentState]) (infer.ReadResponse[ServiceDeploymentArgs, ServiceDeploymentState], error) {
	_, err := client(ctx).GetService(ctx, req.ID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[ServiceDeploymentArgs, ServiceDeploymentState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[ServiceDeploymentArgs, ServiceDeploymentState]{}, err
	}
	inputs := req.Inputs
	if inputs.Service == "" {
		inputs = req.State.ServiceDeploymentArgs
	}
	inputs.Service = req.ID
	state := req.State
	state.ServiceDeploymentArgs = inputs
	return infer.ReadResponse[ServiceDeploymentArgs, ServiceDeploymentState]{ID: req.ID, Inputs: inputs, State: state}, nil
}

// Deleting a request resource neither stops nor removes the service.
func (ServiceDeployment) Delete(context.Context, infer.DeleteRequest[ServiceDeploymentState]) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}
