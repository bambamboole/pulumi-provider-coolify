package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// Deployment triggers a deployment of a Coolify application and tracks it to
// completion. The resource is deployed once per configuration change; a diff
// only appears when the referenced application or the force flag changes.
type Deployment struct{}

type DeploymentArgs struct {
	// UUID of the Coolify application to deploy.
	Application string `pulumi:"application"`
	// Force a rebuild even when there are no new commits.
	Force bool `pulumi:"force,optional"`
	// Deploy a preview of the given pull request instead of the default branch.
	PullRequestID int `pulumi:"pullRequestId,optional"`
	// Override the Docker image tag to deploy.
	DockerTag string `pulumi:"dockerTag,optional"`
}

type DeploymentState struct {
	// UUID of the deployment queue item in Coolify.
	UUID string `pulumi:"uuid"`
	// UUID of the Coolify application.
	Application string `pulumi:"application"`
	// Final status reported by Coolify (finished, failed, cancelled, ...).
	Status string `pulumi:"status"`
	// Git commit that was deployed.
	Commit string `pulumi:"commit,optional"`
	// Whether a forced rebuild was requested.
	Force bool `pulumi:"force"`
	// Pull request this preview belongs to, if any.
	PullRequestID int `pulumi:"pullRequestId,optional"`
	// Docker image tag override used for this deployment, if any.
	DockerTag string `pulumi:"dockerTag,optional"`
}

func (r *Deployment) Annotate(a infer.Annotator) {
	a.SetToken("index", "Deployment")
	a.Describe(&r, "Triggers a deployment of a Coolify application and waits for it to finish.")
}

func (args *DeploymentArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Application, "UUID of the Coolify application to deploy (the uuid output of an Application resource).")
	a.Describe(&args.Force, "Force a rebuild even when there are no new commits.")
	a.Describe(&args.PullRequestID, "Deploy a preview of the given pull request instead of the default branch.")
	a.Describe(&args.DockerTag, "Override the Docker image tag to deploy.")
}

func (Deployment) Create(ctx context.Context, req infer.CreateRequest[DeploymentArgs]) (infer.CreateResponse[DeploymentState], error) {
	if req.DryRun {
		return infer.CreateResponse[DeploymentState]{
			ID:     "pending",
			Output: deploymentStateFromArgs(req.Inputs),
		}, nil
	}
	state, err := runDeployment(ctx, req.Inputs)
	if err != nil {
		return infer.CreateResponse[DeploymentState]{}, err
	}
	return infer.CreateResponse[DeploymentState]{ID: state.UUID, Output: state}, nil
}

func (Deployment) Diff(ctx context.Context, req infer.DiffRequest[DeploymentArgs, DeploymentState]) (infer.DiffResponse, error) {
	changes := req.Inputs.Application != req.State.Application ||
		req.Inputs.Force != req.State.Force ||
		req.Inputs.PullRequestID != req.State.PullRequestID ||
		req.Inputs.DockerTag != req.State.DockerTag
	return infer.DiffResponse{HasChanges: changes}, nil
}

func (Deployment) Update(ctx context.Context, req infer.UpdateRequest[DeploymentArgs, DeploymentState]) (infer.UpdateResponse[DeploymentState], error) {
	if req.DryRun {
		return infer.UpdateResponse[DeploymentState]{
			Output: deploymentStateFromArgs(req.Inputs),
		}, nil
	}
	state, err := runDeployment(ctx, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[DeploymentState]{}, err
	}
	return infer.UpdateResponse[DeploymentState]{Output: state}, nil
}

func (Deployment) Read(ctx context.Context, req infer.ReadRequest[DeploymentArgs, DeploymentState]) (infer.ReadResponse[DeploymentArgs, DeploymentState], error) {
	c := client(ctx)
	deployment, err := c.GetDeployment(ctx, req.ID)
	if err != nil {
		return infer.ReadResponse[DeploymentArgs, DeploymentState]{}, err
	}
	return infer.ReadResponse[DeploymentArgs, DeploymentState]{
		ID:     req.ID,
		Inputs: req.Inputs,
		State: DeploymentState{
			UUID:          deployment.DeploymentUUID,
			Application:   ifEmpty(req.Inputs.Application, req.State.Application),
			Status:        deployment.Status,
			Commit:        deployment.Commit,
			Force:         req.State.Force,
			PullRequestID: req.State.PullRequestID,
			DockerTag:     req.State.DockerTag,
		},
	}, nil
}

// Delete is intentionally a no-op: deployments are immutable history in
// Coolify and are not removed when the resource is deleted.
func (Deployment) Delete(ctx context.Context, req infer.DeleteRequest[DeploymentState]) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}

func runDeployment(ctx context.Context, inputs DeploymentArgs) (DeploymentState, error) {
	c := client(ctx)
	items, err := c.DeployApplication(ctx, inputs.Application, DeployOptions{
		Force:         inputs.Force,
		PullRequestID: inputs.PullRequestID,
		DockerTag:     inputs.DockerTag,
	})
	if err != nil {
		return DeploymentState{}, err
	}
	if len(items) == 0 {
		return DeploymentState{}, fmt.Errorf("coolify returned no deployment for application %q", inputs.Application)
	}
	uuid := items[0].DeploymentUUID

	state, err := waitForDeployment(ctx, uuid)
	if err != nil {
		return DeploymentState{}, err
	}
	state.Application = inputs.Application
	state.Force = inputs.Force
	state.PullRequestID = inputs.PullRequestID
	state.DockerTag = inputs.DockerTag
	return state, nil
}

func deploymentStateFromArgs(inputs DeploymentArgs) DeploymentState {
	return DeploymentState{
		UUID:          "pending",
		Application:   inputs.Application,
		Force:         inputs.Force,
		PullRequestID: inputs.PullRequestID,
		DockerTag:     inputs.DockerTag,
	}
}

func waitForDeployment(ctx context.Context, uuid string) (DeploymentState, error) {
	c := client(ctx)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	deadline := time.Now().Add(30 * time.Minute)

	for {
		deployment, err := c.GetDeployment(ctx, uuid)
		if err != nil {
			if NotFound(err) {
				// The queue item may not be visible immediately; keep waiting.
				deployment = CoolifyDeployment{DeploymentUUID: uuid}
			} else {
				return DeploymentState{}, err
			}
		}

		if isTerminalStatus(deployment.Status) {
			if deployment.Status == "success" || deployment.Status == "finished" {
				return DeploymentState{
					UUID:   uuid,
					Status: deployment.Status,
					Commit: deployment.Commit,
				}, nil
			}
			return DeploymentState{}, fmt.Errorf("deployment %q finished with status %q", uuid, deployment.Status)
		}
		if time.Now().After(deadline) {
			return DeploymentState{}, fmt.Errorf("deployment %q did not finish within 30 minutes (last status %q)", uuid, deployment.Status)
		}

		select {
		case <-ctx.Done():
			return DeploymentState{}, ctx.Err()
		case <-ticker.C:
		}
	}
}

func isTerminalStatus(status string) bool {
	switch status {
	case "success", "finished", "failed", "error", "cancelled", "cancelled-by-user":
		return true
	}
	return false
}
