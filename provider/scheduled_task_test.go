package provider

import (
	"context"
	"testing"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

func TestCreateScheduledTaskCreatesAdoptsAndPatches(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	ctx := context.Background()
	projectUUID := fake.addProject("Main", "production")
	appUUID := fake.addApplication(map[string]any{"name": "app", "environment_id": fake.environmentID(projectUUID, "production"), "settings": map[string]any{}})

	args := ScheduledTaskArgs{ApplicationUUID: appUUID, Name: "backup", Command: "php artisan backup:run", Frequency: "daily", Enabled: true}
	task, err := createScheduledTask(ctx, c, args)
	if err != nil {
		t.Fatalf("createScheduledTask: %v", err)
	}
	if *task.Name != "backup" || *task.Frequency != "daily" || !*task.Enabled || task.Timeout != nil {
		t.Fatalf("unexpected task: %+v", task)
	}

	args.Frequency = "0 3 * * *"
	args.Enabled = false
	adopted, err := createScheduledTask(ctx, c, args)
	if err != nil {
		t.Fatalf("second createScheduledTask: %v", err)
	}
	if *adopted.Uuid != *task.Uuid || *adopted.Frequency != "0 3 * * *" || *adopted.Enabled {
		t.Fatalf("task was not adopted and patched: %+v", adopted)
	}
	if fake.countRequests("POST", "/api/v1/applications/"+appUUID+"/scheduled-tasks") != 1 {
		t.Fatalf("task was recreated: %v", fake.requests)
	}

	got, err := c.GetScheduledTask(ctx, appUUID, *task.Uuid)
	if err != nil || *got.Uuid != *task.Uuid {
		t.Fatalf("GetScheduledTask: %+v %v", got, err)
	}
	if _, err := c.GetScheduledTask(ctx, appUUID, "missing"); !coolify.IsNotFound(err) {
		t.Fatalf("missing task must be not found, got %v", err)
	}
	if err := c.DeleteScheduledTask(ctx, appUUID, *task.Uuid); err != nil {
		t.Fatalf("DeleteScheduledTask: %v", err)
	}
	if tasks, _ := c.ListScheduledTasks(ctx, appUUID); len(tasks) != 0 {
		t.Fatalf("task not deleted: %+v", tasks)
	}
}
