package coolify

import (
	"context"
	"net/http"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

func (c *Client) ListScheduledTasks(ctx context.Context, applicationUUID string) ([]api.ScheduledTask, error) {
	return decode[[]api.ScheduledTask](c.api.ListScheduledTasksByApplicationUuid(ctx, applicationUUID))
}

// GetScheduledTask finds a task by UUID. The API has no single-task endpoint,
// so it lists the application's tasks and returns a 404 APIError when missing.
func (c *Client) GetScheduledTask(ctx context.Context, applicationUUID, taskUUID string) (api.ScheduledTask, error) {
	tasks, err := c.ListScheduledTasks(ctx, applicationUUID)
	if err != nil {
		return api.ScheduledTask{}, err
	}
	for _, task := range tasks {
		if Deref(task.Uuid) == taskUUID {
			return task, nil
		}
	}
	return api.ScheduledTask{}, &APIError{
		Status: http.StatusNotFound,
		Method: http.MethodGet,
		Path:   apiPath + "/applications/" + applicationUUID + "/scheduled-tasks/" + taskUUID,
		Body:   `{"message":"Scheduled task not found."}`,
	}
}

func (c *Client) CreateScheduledTask(ctx context.Context, applicationUUID string, body api.CreateScheduledTaskByApplicationUuidJSONRequestBody) (api.ScheduledTask, error) {
	return decode[api.ScheduledTask](c.api.CreateScheduledTaskByApplicationUuid(ctx, applicationUUID, body))
}

func (c *Client) UpdateScheduledTask(ctx context.Context, applicationUUID, taskUUID string, body api.UpdateScheduledTaskByApplicationUuidJSONRequestBody) (api.ScheduledTask, error) {
	return decode[api.ScheduledTask](c.api.UpdateScheduledTaskByApplicationUuid(ctx, applicationUUID, taskUUID, body))
}

func (c *Client) DeleteScheduledTask(ctx context.Context, applicationUUID, taskUUID string) error {
	return check(c.api.DeleteScheduledTaskByApplicationUuid(ctx, applicationUUID, taskUUID))
}
