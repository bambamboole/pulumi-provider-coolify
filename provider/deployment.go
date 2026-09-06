package provider

import (
	"context"
	"fmt"
	"time"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

var (
	deploymentPollInterval = 10 * time.Second
	deploymentTimeout      = 30 * time.Minute
	// deploymentNotFoundGrace bounds how long a freshly queued deployment may
	// stay invisible to the API before the wait gives up.
	deploymentNotFoundGrace = 2 * time.Minute
)

// Deployment triggers a deployment of a Coolify application and waits for it
// to finish. It deploys again whenever one of its inputs changes; use triggers
// to force a redeploy without changing anything else.
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
	// Arbitrary values that trigger a redeploy when they change, e.g. an image digest.
	Triggers []string `pulumi:"triggers,optional"`
}

type DeploymentState struct {
	DeploymentArgs
	// UUID of the deployment in Coolify.
	UUID string `pulumi:"uuid"`
	// Final status reported by Coolify.
	Status string `pulumi:"status"`
	// Git commit that was deployed, if any.
	Commit string `pulumi:"commit"`
}

func (r *Deployment) Annotate(a infer.Annotator) {
	a.SetToken("index", "Deployment")
	a.Describe(&r, "Triggers a deployment of a Coolify application and waits for it to finish. Deploys again whenever an input changes.")
}

func (args *DeploymentArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Application, "UUID of the Coolify application to deploy (the uuid output of an Application resource).")
	a.Describe(&args.Force, "Force a rebuild even when there are no new commits.")
	a.Describe(&args.PullRequestID, "Deploy a preview of the given pull request instead of the default branch.")
	a.Describe(&args.DockerTag, "Override the Docker image tag to deploy.")
	a.Describe(&args.Triggers, "Arbitrary values that trigger a redeploy when they change, e.g. an image digest or a version.")
}

func (state *DeploymentState) Annotate(a infer.Annotator) {
	a.Describe(&state.UUID, "UUID of the deployment in Coolify.")
	a.Describe(&state.Status, "Final status reported by Coolify.")
	a.Describe(&state.Commit, "Git commit that was deployed, if any.")
}

func (Deployment) Create(ctx context.Context, req infer.CreateRequest[DeploymentArgs]) (infer.CreateResponse[DeploymentState], error) {
	if req.DryRun {
		return infer.CreateResponse[DeploymentState]{Output: DeploymentState{DeploymentArgs: req.Inputs}}, nil
	}
	state, err := runDeployment(ctx, client(ctx), req.Inputs)
	if err != nil {
		return infer.CreateResponse[DeploymentState]{}, err
	}
	return infer.CreateResponse[DeploymentState]{ID: state.UUID, Output: state}, nil
}

func (Deployment) Diff(ctx context.Context, req infer.DiffRequest[DeploymentArgs, DeploymentState]) (infer.DiffResponse, error) {
	return diffResponse(diffArgs(req.State.DeploymentArgs, req.Inputs), false), nil
}

func (Deployment) Update(ctx context.Context, req infer.UpdateRequest[DeploymentArgs, DeploymentState]) (infer.UpdateResponse[DeploymentState], error) {
	if req.DryRun {
		state := req.State
		state.DeploymentArgs = req.Inputs
		return infer.UpdateResponse[DeploymentState]{Output: state}, nil
	}
	state, err := runDeployment(ctx, client(ctx), req.Inputs)
	if err != nil {
		return infer.UpdateResponse[DeploymentState]{}, err
	}
	return infer.UpdateResponse[DeploymentState]{Output: state}, nil
}

// Read refreshes the status. A deployment Coolify has pruned from its history
// keeps its recorded state instead of being dropped, which would redeploy.
func (Deployment) Read(ctx context.Context, req infer.ReadRequest[DeploymentArgs, DeploymentState]) (infer.ReadResponse[DeploymentArgs, DeploymentState], error) {
	state := req.State
	deployment, err := client(ctx).GetDeployment(ctx, req.ID)
	if err != nil && !coolify.IsNotFound(err) {
		return infer.ReadResponse[DeploymentArgs, DeploymentState]{}, err
	}
	if err == nil {
		state.Status = coolify.Deref(deployment.Status)
		state.Commit = coolify.Deref(deployment.Commit)
	}
	return infer.ReadResponse[DeploymentArgs, DeploymentState]{ID: req.ID, Inputs: req.Inputs, State: state}, nil
}

// Delete is a no-op: deployments are immutable history in Coolify.
func (Deployment) Delete(context.Context, infer.DeleteRequest[DeploymentState]) (infer.DeleteResponse, error) {
	return infer.DeleteResponse{}, nil
}

func runDeployment(ctx context.Context, c *coolify.Client, inputs DeploymentArgs) (DeploymentState, error) {
	items, err := c.DeployApplication(ctx, inputs.Application, coolify.DeployOptions{
		Force:         inputs.Force,
		PullRequestID: inputs.PullRequestID,
		DockerTag:     inputs.DockerTag,
	})
	if err != nil {
		return DeploymentState{}, err
	}
	if len(items) == 0 {
		return DeploymentState{}, fmt.Errorf("coolify queued no deployment for application %q", inputs.Application)
	}
	uuid := items[0].DeploymentUUID
	status, commit, err := waitForDeployment(ctx, c, uuid)
	if err != nil {
		return DeploymentState{}, err
	}
	return DeploymentState{DeploymentArgs: inputs, UUID: uuid, Status: status, Commit: commit}, nil
}

// waitForDeployment polls the deployment until it reaches a terminal status.
func waitForDeployment(ctx context.Context, c *coolify.Client, uuid string) (status, commit string, err error) {
	ticker := time.NewTicker(deploymentPollInterval)
	defer ticker.Stop()
	started := time.Now()

	for {
		deployment, err := c.GetDeployment(ctx, uuid)
		switch {
		case coolify.IsNotFound(err):
			// The queue item may not be visible immediately.
			if time.Since(started) > deploymentNotFoundGrace {
				return "", "", fmt.Errorf("deployment %q was not found within %s", uuid, deploymentNotFoundGrace)
			}
		case err != nil:
			return "", "", err
		default:
			status := coolify.Deref(deployment.Status)
			if isTerminalStatus(status) {
				if isSuccessStatus(status) {
					return status, coolify.Deref(deployment.Commit), nil
				}
				return "", "", fmt.Errorf("deployment %q finished with status %q", uuid, status)
			}
			if time.Since(started) > deploymentTimeout {
				return "", "", fmt.Errorf("deployment %q did not finish within %s (last status %q)", uuid, deploymentTimeout, status)
			}
		}

		select {
		case <-ctx.Done():
			return "", "", ctx.Err()
		case <-ticker.C:
		}
	}
}

func isSuccessStatus(status string) bool {
	return status == "success" || status == "finished"
}

func isTerminalStatus(status string) bool {
	switch status {
	case "success", "finished", "failed", "error", "cancelled", "cancelled-by-user":
		return true
	}
	return false
}
