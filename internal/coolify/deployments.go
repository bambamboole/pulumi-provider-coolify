package coolify

import (
	"context"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// DeployOptions configures a deployment trigger.
type DeployOptions struct {
	// Force rebuilds even when there are no new commits.
	Force bool
	// PullRequestID deploys a preview of the given pull request.
	PullRequestID int
	// DockerTag overrides the Docker image tag to deploy.
	DockerTag string
}

// QueueItem is a deployment Coolify queued in response to a deploy request.
type QueueItem struct {
	Message        string `json:"message"`
	ResourceUUID   string `json:"resource_uuid"`
	DeploymentUUID string `json:"deployment_uuid"`
}

// DeployApplication triggers a deployment of the application and returns the
// queued deployments.
func (c *Client) DeployApplication(ctx context.Context, applicationUUID string, opts DeployOptions) ([]QueueItem, error) {
	params := api.DeployByTagOrUuidParams{
		Uuid:          &applicationUUID,
		Force:         PtrIfNonZero(opts.Force),
		PullRequestId: PtrIfNonZero(opts.PullRequestID),
		DockerTag:     PtrIfNonZero(opts.DockerTag),
	}
	out, err := decode[struct {
		Deployments []QueueItem `json:"deployments"`
	}](c.api.DeployByTagOrUuid(ctx, &params))
	return out.Deployments, err
}

func (c *Client) GetDeployment(ctx context.Context, uuid string) (api.ApplicationDeploymentQueue, error) {
	return decode[api.ApplicationDeploymentQueue](c.api.GetDeploymentByUuid(ctx, uuid))
}
