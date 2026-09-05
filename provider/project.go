package provider

import (
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"
)

// Project manages a Coolify project and its environments.
type Project struct{}

type ProjectArgs struct {
	// Name of the project.
	Name string `pulumi:"name"`
	// Description of the project.
	Description string `pulumi:"description,optional"`
	// Environments that must exist in the project. Environments are only ever
	// added; environments not listed here are left untouched.
	Environments []string `pulumi:"environments,optional"`
}

type ProjectState struct {
	// UUID of the project in Coolify.
	UUID string `pulumi:"uuid"`
	// Name of the project.
	Name string `pulumi:"name"`
	// Description of the project.
	Description string `pulumi:"description"`
	// Environments managed by Pulumi.
	Environments []string `pulumi:"environments"`
}

func (r *Project) Annotate(a infer.Annotator) {
	a.SetToken("index", "Project")
	a.Describe(&r, "A Coolify project with its environments.")
}

func (args *ProjectArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Name, "Name of the project.")
	a.Describe(&args.Description, "Description of the project.")
	a.Describe(&args.Environments, "Environments that must exist in the project. Only added; never removed.")
}

func (state *ProjectState) Annotate(a infer.Annotator) {
	a.Describe(&state.UUID, "UUID of the project in Coolify.")
}

func (Project) Create(ctx context.Context, req infer.CreateRequest[ProjectArgs]) (infer.CreateResponse[ProjectState], error) {
	if req.DryRun {
		return infer.CreateResponse[ProjectState]{ID: "pending", Output: projectPlaceholder(req.Inputs)}, nil
	}
	c := client(ctx)
	state, err := syncProject(ctx, c, req.Inputs)
	if err != nil {
		return infer.CreateResponse[ProjectState]{}, err
	}
	return infer.CreateResponse[ProjectState]{ID: state.UUID, Output: state}, nil
}

func (Project) Diff(ctx context.Context, req infer.DiffRequest[ProjectArgs, ProjectState]) (infer.DiffResponse, error) {
	changes := projectChanged(req.State, req.Inputs)
	return infer.DiffResponse{HasChanges: changes}, nil
}

func (Project) Update(ctx context.Context, req infer.UpdateRequest[ProjectArgs, ProjectState]) (infer.UpdateResponse[ProjectState], error) {
	if req.DryRun {
		return infer.UpdateResponse[ProjectState]{
			Output: projectState(req.State.UUID, req.Inputs),
		}, nil
	}
	c := client(ctx)
	state, err := syncProject(ctx, c, req.Inputs)
	if err != nil {
		return infer.UpdateResponse[ProjectState]{}, err
	}
	return infer.UpdateResponse[ProjectState]{Output: state}, nil
}

func (Project) Read(ctx context.Context, req infer.ReadRequest[ProjectArgs, ProjectState]) (infer.ReadResponse[ProjectArgs, ProjectState], error) {
	c := client(ctx)
	project, err := c.GetProject(ctx, req.ID)
	if err != nil {
		return infer.ReadResponse[ProjectArgs, ProjectState]{}, err
	}
	environments, err := c.ListEnvironments(ctx, project.UUID)
	if err != nil {
		return infer.ReadResponse[ProjectArgs, ProjectState]{}, err
	}
	names := make([]string, 0, len(environments))
	for _, environment := range environments {
		names = append(names, environment.Name)
	}
	inputs := req.Inputs
	if inputs.Name == "" {
		inputs.Name = project.Name
		inputs.Description = project.Description
	}
	return infer.ReadResponse[ProjectArgs, ProjectState]{
		ID:     req.ID,
		Inputs: inputs,
		State: ProjectState{
			UUID:         project.UUID,
			Name:         project.Name,
			Description:  project.Description,
			Environments: uniqueSorted(names),
		},
	}, nil
}

func (Project) Delete(ctx context.Context, req infer.DeleteRequest[ProjectState]) (infer.DeleteResponse, error) {
	c := client(ctx)
	for _, environment := range req.State.Environments {
		if err := c.DeleteEnvironment(ctx, req.State.UUID, environment); err != nil && !NotFound(err) {
			return infer.DeleteResponse{}, err
		}
	}
	if err := c.DeleteProject(ctx, req.State.UUID); err != nil && !NotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}

func syncProject(ctx context.Context, c *Client, inputs ProjectArgs) (ProjectState, error) {

	projects, err := c.ListProjects(ctx)
	if err != nil {
		return ProjectState{}, err
	}
	var existing *CoolifyProject
	for i := range projects {
		if projects[i].Name == inputs.Name {
			existing = &projects[i]
			break
		}
	}

	uuid := ""
	if existing == nil {
		uuid, err = c.CreateProject(ctx, inputs.Name, inputs.Description)
		if err != nil {
			return ProjectState{}, err
		}
	} else {
		uuid = existing.UUID
		if existing.Name != inputs.Name || existing.Description != inputs.Description {
			if err := c.UpdateProject(ctx, uuid, inputs.Name, inputs.Description); err != nil {
				return ProjectState{}, err
			}
		}
	}

	present := map[string]struct{}{}
	environments, err := c.ListEnvironments(ctx, uuid)
	if err != nil {
		return ProjectState{}, err
	}
	for _, environment := range environments {
		present[environment.Name] = struct{}{}
	}
	for _, name := range uniquePreservingOrder(inputs.Environments) {
		if _, ok := present[name]; ok {
			continue
		}
		if err := c.CreateEnvironment(ctx, uuid, name); err != nil && !Conflict(err) {
			return ProjectState{}, err
		}
	}

	return projectState(uuid, inputs), nil
}

func projectState(uuid string, inputs ProjectArgs) ProjectState {
	return ProjectState{
		UUID:         uuid,
		Name:         inputs.Name,
		Description:  inputs.Description,
		Environments: uniqueSorted(inputs.Environments),
	}
}

func projectPlaceholder(inputs ProjectArgs) ProjectState {
	return ProjectState{
		UUID:         "pending",
		Name:         inputs.Name,
		Description:  inputs.Description,
		Environments: uniqueSorted(inputs.Environments),
	}
}

func projectChanged(state ProjectState, inputs ProjectArgs) bool {
	return state.Name != inputs.Name ||
		state.Description != inputs.Description ||
		len(state.Environments) != len(uniqueSorted(inputs.Environments)) ||
		!sameStrings(state.Environments, uniqueSorted(inputs.Environments))
}

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
