package provider

import (
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// Project manages a Coolify project and the environments declared on it.
type Project struct{}

type ProjectArgs struct {
	// Name of the project. An existing project with this name is adopted.
	Name string `pulumi:"name"`
	// Description of the project.
	Description string `pulumi:"description,optional"`
	// Environments that must exist in the project. Environments are only ever
	// added; environments not listed here are left untouched.
	Environments []string `pulumi:"environments,optional"`
}

type ProjectState struct {
	ProjectArgs
	// UUID of the project in Coolify.
	UUID string `pulumi:"uuid"`
}

func (r *Project) Annotate(a infer.Annotator) {
	a.SetToken("index", "Project")
	a.Describe(&r, "A Coolify project with its environments. An existing project with the same name is adopted on create.")
}

func (args *ProjectArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "Name of the project. An existing project with this name is adopted.")
	a.Describe(&args.Description, "Description of the project.")
	a.Describe(&args.Environments, "Environments that must exist in the project. Only added; never removed.")
}

func (state *ProjectState) Annotate(a infer.Annotator) {
	a.Describe(&state.UUID, "UUID of the project in Coolify.")
}

func (Project) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[ProjectArgs], error) {
	args, failures, err := infer.DefaultCheck[ProjectArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[ProjectArgs]{}, err
	}
	args.Environments = uniquePreservingOrder(args.Environments)
	return infer.CheckResponse[ProjectArgs]{Inputs: args, Failures: failures}, nil
}

func (Project) Create(ctx context.Context, req infer.CreateRequest[ProjectArgs]) (infer.CreateResponse[ProjectState], error) {
	state := ProjectState{ProjectArgs: req.Inputs}
	if req.DryRun {
		return infer.CreateResponse[ProjectState]{Output: state}, nil
	}
	uuid, err := createProject(ctx, client(ctx), req.Inputs)
	if err != nil {
		return infer.CreateResponse[ProjectState]{}, err
	}
	state.UUID = uuid
	return infer.CreateResponse[ProjectState]{ID: uuid, Output: state}, nil
}

func (Project) Diff(ctx context.Context, req infer.DiffRequest[ProjectArgs, ProjectState]) (infer.DiffResponse, error) {
	return diffResponse(diffArgs(req.State.ProjectArgs, req.Inputs), true), nil
}

func (Project) Update(ctx context.Context, req infer.UpdateRequest[ProjectArgs, ProjectState]) (infer.UpdateResponse[ProjectState], error) {
	state := ProjectState{ProjectArgs: req.Inputs, UUID: req.ID}
	if req.DryRun {
		return infer.UpdateResponse[ProjectState]{Output: state}, nil
	}
	c := client(ctx)
	if err := c.UpdateProject(ctx, req.ID, api.UpdateProjectByUuidJSONRequestBody{
		Name:        &req.Inputs.Name,
		Description: &req.Inputs.Description,
	}); err != nil {
		return infer.UpdateResponse[ProjectState]{}, err
	}
	if err := ensureEnvironments(ctx, c, req.ID, req.Inputs.Environments); err != nil {
		return infer.UpdateResponse[ProjectState]{}, err
	}
	return infer.UpdateResponse[ProjectState]{Output: state}, nil
}

func (Project) Read(ctx context.Context, req infer.ReadRequest[ProjectArgs, ProjectState]) (infer.ReadResponse[ProjectArgs, ProjectState], error) {
	c := client(ctx)
	project, err := c.GetProject(ctx, req.ID)
	if coolify.IsNotFound(err) {
		return infer.ReadResponse[ProjectArgs, ProjectState]{}, nil
	}
	if err != nil {
		return infer.ReadResponse[ProjectArgs, ProjectState]{}, err
	}
	environments, err := c.ListEnvironments(ctx, req.ID)
	if err != nil {
		return infer.ReadResponse[ProjectArgs, ProjectState]{}, err
	}
	existing := map[string]bool{}
	for _, environment := range environments {
		existing[environment.Name] = true
	}

	inputs := ProjectArgs{
		Name:        coolify.Deref(project.Name),
		Description: coolify.Deref(project.Description),
	}
	if req.Inputs.Name == "" {
		// Import: adopt every environment that exists.
		for _, environment := range environments {
			inputs.Environments = append(inputs.Environments, environment.Name)
		}
	} else {
		// Refresh: keep the declared environments that still exist so a
		// deleted one is recreated on the next update.
		for _, name := range req.Inputs.Environments {
			if existing[name] {
				inputs.Environments = append(inputs.Environments, name)
			}
		}
	}
	return infer.ReadResponse[ProjectArgs, ProjectState]{
		ID:     req.ID,
		Inputs: inputs,
		State:  ProjectState{ProjectArgs: inputs, UUID: req.ID},
	}, nil
}

func (Project) Delete(ctx context.Context, req infer.DeleteRequest[ProjectState]) (infer.DeleteResponse, error) {
	c := client(ctx)
	for _, environment := range req.State.Environments {
		if err := c.DeleteEnvironment(ctx, req.ID, environment); err != nil && !coolify.IsNotFound(err) {
			return infer.DeleteResponse{}, err
		}
	}
	if err := c.DeleteProject(ctx, req.ID); err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

// createProject adopts the project with the given name or creates it, then
// makes sure the declared environments exist. It returns the project UUID.
func createProject(ctx context.Context, c *coolify.Client, inputs ProjectArgs) (string, error) {
	projects, err := c.ListProjects(ctx)
	if err != nil {
		return "", err
	}
	uuid := ""
	for _, project := range projects {
		if coolify.Deref(project.Name) == inputs.Name {
			uuid = coolify.Deref(project.Uuid)
			break
		}
	}
	if uuid == "" {
		uuid, err = c.CreateProject(ctx, inputs.Name, inputs.Description)
		if err != nil {
			return "", err
		}
	} else if err := c.UpdateProject(ctx, uuid, api.UpdateProjectByUuidJSONRequestBody{
		Name:        &inputs.Name,
		Description: &inputs.Description,
	}); err != nil {
		return "", err
	}
	if err := ensureEnvironments(ctx, c, uuid, inputs.Environments); err != nil {
		return "", err
	}
	return uuid, nil
}

// ensureEnvironments creates the declared environments that do not exist yet.
func ensureEnvironments(ctx context.Context, c *coolify.Client, projectUUID string, names []string) error {
	environments, err := c.ListEnvironments(ctx, projectUUID)
	if err != nil {
		return err
	}
	present := map[string]bool{}
	for _, environment := range environments {
		present[environment.Name] = true
	}
	for _, name := range names {
		if present[name] {
			continue
		}
		if _, err := c.CreateEnvironment(ctx, projectUUID, name); err != nil && !coolify.IsConflict(err) {
			return err
		}
	}
	return nil
}
