package provider

import (
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// ScheduledTask manages a scheduled (cron) task on a Coolify application.
type ScheduledTask struct{}

type ScheduledTaskArgs struct {
	// UUID of the Coolify application the task runs on.
	ApplicationUUID string `pulumi:"applicationUuid"`
	// Name of the task. An existing task with this name on the application is adopted.
	Name string `pulumi:"name"`
	// Command to execute.
	Command string `pulumi:"command"`
	// Cron expression or Coolify shorthand such as "daily", "hourly" or "@every 5m".
	Frequency string `pulumi:"frequency"`
	// Container to run the command in. Defaults to the application's container.
	Container string `pulumi:"container,optional"`
	// Timeout in seconds. Leave unset to keep Coolify's default.
	Timeout int `pulumi:"timeout,optional"`
	// Whether the task is enabled.
	Enabled bool `pulumi:"enabled,optional"`
}

type ScheduledTaskState struct {
	ScheduledTaskArgs
	// UUID of the task in Coolify.
	UUID string `pulumi:"uuid"`
}

func (r *ScheduledTask) Annotate(a infer.Annotator) {
	a.SetToken("index", "ScheduledTask")
	a.Describe(&r, "A scheduled task (cron job) on a Coolify application. An existing task with the same name on the application is adopted on create.")
}

func (args *ScheduledTaskArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.ApplicationUUID, "UUID of the Coolify application the task runs on (the uuid output of an Application resource).")
	a.Describe(&args.Name, "Name of the task. An existing task with this name on the application is adopted.")
	a.Describe(&args.Command, "Command to execute.")
	a.Describe(&args.Frequency, `Cron expression or Coolify shorthand such as "daily", "hourly" or "@every 5m".`)
	a.Describe(&args.Container, "Container to run the command in. Defaults to the application's container.")
	a.Describe(&args.Timeout, "Timeout in seconds. Leave unset to keep Coolify's default.")
	a.Describe(&args.Enabled, "Whether the task is enabled.")
	a.SetDefault(&args.Enabled, true)
}

func (state *ScheduledTaskState) Annotate(a infer.Annotator) {
	a.Describe(&state.UUID, "UUID of the task in Coolify.")
}

func (ScheduledTask) Create(ctx context.Context, req infer.CreateRequest[ScheduledTaskArgs]) (infer.CreateResponse[ScheduledTaskState], error) {
	if req.DryRun {
		return infer.CreateResponse[ScheduledTaskState]{Output: ScheduledTaskState{ScheduledTaskArgs: req.Inputs}}, nil
	}
	task, err := createScheduledTask(ctx, client(ctx), req.Inputs)
	if err != nil {
		return infer.CreateResponse[ScheduledTaskState]{}, err
	}
	return infer.CreateResponse[ScheduledTaskState]{ID: coolify.Deref(task.Uuid), Output: scheduledTaskState(req.Inputs, task)}, nil
}

func (ScheduledTask) Diff(ctx context.Context, req infer.DiffRequest[ScheduledTaskArgs, ScheduledTaskState]) (infer.DiffResponse, error) {
	diff := diffArgs(req.State.ScheduledTaskArgs, req.Inputs, "applicationUuid")
	return diffResponse(diff, req.State.Name == req.Inputs.Name), nil
}

func (ScheduledTask) Update(ctx context.Context, req infer.UpdateRequest[ScheduledTaskArgs, ScheduledTaskState]) (infer.UpdateResponse[ScheduledTaskState], error) {
	if req.DryRun {
		return infer.UpdateResponse[ScheduledTaskState]{Output: ScheduledTaskState{ScheduledTaskArgs: req.Inputs, UUID: req.ID}}, nil
	}
	c := client(ctx)
	current, err := c.GetScheduledTask(ctx, req.Inputs.ApplicationUUID, req.ID)
	if err != nil {
		return infer.UpdateResponse[ScheduledTaskState]{}, err
	}
	task, err := applyScheduledTask(ctx, c, current, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[ScheduledTaskState]{}, err
	}
	return infer.UpdateResponse[ScheduledTaskState]{Output: scheduledTaskState(req.Inputs, task)}, nil
}

func (ScheduledTask) Read(ctx context.Context, req infer.ReadRequest[ScheduledTaskArgs, ScheduledTaskState]) (infer.ReadResponse[ScheduledTaskArgs, ScheduledTaskState], error) {
	applicationUUID := firstNonEmpty(req.Inputs.ApplicationUUID, req.State.ApplicationUUID)
	task, err := client(ctx).GetScheduledTask(ctx, applicationUUID, req.ID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[ScheduledTaskArgs, ScheduledTaskState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[ScheduledTaskArgs, ScheduledTaskState]{}, err
	}
	inputs := scheduledTaskInputs(req.Inputs, task)
	inputs.ApplicationUUID = applicationUUID
	return infer.ReadResponse[ScheduledTaskArgs, ScheduledTaskState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  scheduledTaskState(inputs, task),
	}, nil
}

func (ScheduledTask) Delete(ctx context.Context, req infer.DeleteRequest[ScheduledTaskState]) (infer.DeleteResponse, error) {
	if err := client(ctx).DeleteScheduledTask(ctx, req.State.ApplicationUUID, req.ID); err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

// createScheduledTask adopts the task with the same name on the application or
// creates it, and reconciles its settings with the inputs.
func createScheduledTask(ctx context.Context, c *coolify.Client, inputs ScheduledTaskArgs) (api.ScheduledTask, error) {
	tasks, err := c.ListScheduledTasks(ctx, inputs.ApplicationUUID)
	if err != nil {
		return api.ScheduledTask{}, err
	}
	for _, task := range tasks {
		if coolify.Deref(task.Name) == inputs.Name {
			return applyScheduledTask(ctx, c, task, inputs)
		}
	}
	return c.CreateScheduledTask(ctx, inputs.ApplicationUUID, api.CreateScheduledTaskByApplicationUuidJSONRequestBody{
		Name:      inputs.Name,
		Command:   inputs.Command,
		Frequency: inputs.Frequency,
		Container: coolify.PtrIfNonZero(inputs.Container),
		Timeout:   coolify.PtrIfNonZero(inputs.Timeout),
		Enabled:   &inputs.Enabled,
	})
}

// applyScheduledTask patches the fields of current that differ from the inputs.
func applyScheduledTask(ctx context.Context, c *coolify.Client, current api.ScheduledTask, inputs ScheduledTaskArgs) (api.ScheduledTask, error) {
	var body api.UpdateScheduledTaskByApplicationUuidJSONRequestBody
	var patch patch
	patch.str(&body.Name, inputs.Name, coolify.Deref(current.Name))
	patch.str(&body.Command, inputs.Command, coolify.Deref(current.Command))
	patch.str(&body.Frequency, inputs.Frequency, coolify.Deref(current.Frequency))
	patch.str(&body.Container, inputs.Container, coolify.Deref(current.Container))
	patch.integer(&body.Timeout, inputs.Timeout, current.Timeout)
	patch.boolean(&body.Enabled, inputs.Enabled, coolify.Deref(current.Enabled))
	if !patch.changed {
		return current, nil
	}
	return c.UpdateScheduledTask(ctx, inputs.ApplicationUUID, coolify.Deref(current.Uuid), body)
}

// scheduledTaskInputs derives the inputs from the task Coolify reports, keeping
// unmanaged optional inputs.
func scheduledTaskInputs(previous ScheduledTaskArgs, task api.ScheduledTask) ScheduledTaskArgs {
	inputs := previous
	inputs.Name = coolify.Deref(task.Name)
	inputs.Command = coolify.Deref(task.Command)
	inputs.Frequency = coolify.Deref(task.Frequency)
	inputs.Container = ifSet(previous.Container, coolify.Deref(task.Container))
	inputs.Timeout = ifSet(previous.Timeout, coolify.Deref(task.Timeout))
	inputs.Enabled = coolify.Deref(task.Enabled)
	return inputs
}

func scheduledTaskState(inputs ScheduledTaskArgs, task api.ScheduledTask) ScheduledTaskState {
	return ScheduledTaskState{ScheduledTaskArgs: inputs, UUID: coolify.Deref(task.Uuid)}
}
