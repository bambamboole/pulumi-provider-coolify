package provider

import (
	"context"

	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

// TeamSharedVariable manages one shared variable for the API token's team.
type TeamSharedVariable struct{}

type TeamSharedVariableArgs struct {
	SharedVariableArgs
}

type TeamSharedVariableState struct {
	TeamSharedVariableArgs
	SharedVariableOutputs
}

func (r *TeamSharedVariable) Annotate(a infer.Annotator) {
	a.SetToken("index", "TeamSharedVariable")
	a.Describe(&r, "A shared variable for the API token's team on Coolify v4.3.0+. Adopts an existing key in this scope and updates declared settings in place. Delete removes the variable. Import using team/<id>; URL-encode path components containing slashes.")
}

func (args TeamSharedVariableArgs) sharedFields() SharedVariableArgs { return args.SharedVariableArgs }
func (args TeamSharedVariableArgs) sharedScope() coolify.SharedVariableScope {
	return coolify.SharedVariableScope{Type: "team"}
}

func (args TeamSharedVariableArgs) withSharedFields(fields SharedVariableArgs, _ coolify.SharedVariableScope) TeamSharedVariableArgs {
	args.SharedVariableArgs = fields
	return args
}

func teamSharedVariableState(args TeamSharedVariableArgs, id int) TeamSharedVariableState {
	return TeamSharedVariableState{TeamSharedVariableArgs: args, SharedVariableOutputs: sharedVariableOutputs(args.sharedScope(), args.Key, id)}
}

func (TeamSharedVariable) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[TeamSharedVariableArgs], error) {
	return checkSharedVariable[TeamSharedVariableArgs](ctx, req)
}

func (TeamSharedVariable) Create(ctx context.Context, req infer.CreateRequest[TeamSharedVariableArgs]) (infer.CreateResponse[TeamSharedVariableState], error) {
	return createSharedVariable(ctx, req, teamSharedVariableState)
}

func (TeamSharedVariable) Diff(ctx context.Context, req infer.DiffRequest[TeamSharedVariableArgs, TeamSharedVariableState]) (infer.DiffResponse, error) {
	return diffSharedVariable(req.State.TeamSharedVariableArgs, req.Inputs), nil
}

func (TeamSharedVariable) Update(ctx context.Context, req infer.UpdateRequest[TeamSharedVariableArgs, TeamSharedVariableState]) (infer.UpdateResponse[TeamSharedVariableState], error) {
	return updateSharedVariable(ctx, req, req.State.TeamSharedVariableArgs, teamSharedVariableState)
}

func (TeamSharedVariable) Read(ctx context.Context, req infer.ReadRequest[TeamSharedVariableArgs, TeamSharedVariableState]) (infer.ReadResponse[TeamSharedVariableArgs, TeamSharedVariableState], error) {
	return readSharedVariable(ctx, req, req.State.TeamSharedVariableArgs, req.State.VariableID == 0, teamSharedVariableState)
}

func (TeamSharedVariable) Delete(ctx context.Context, req infer.DeleteRequest[TeamSharedVariableState]) (infer.DeleteResponse, error) {
	return deleteSharedVariable(ctx, "team", req.ID)
}

func (TeamSharedVariable) WireDependencies(f infer.FieldSelector, args *TeamSharedVariableArgs, state *TeamSharedVariableState) {
	f.OutputField(&state.Reference).DependsOn(f.InputField(&args.Key))
	f.OutputField(&state.VariableID).NeverSecret()
}

// ProjectSharedVariable manages one shared variable for a project.
type ProjectSharedVariable struct{}

type ProjectSharedVariableArgs struct {
	SharedVariableArgs
	ProjectUUID string `pulumi:"projectUuid"`
}

type ProjectSharedVariableState struct {
	ProjectSharedVariableArgs
	SharedVariableOutputs
}

func (r *ProjectSharedVariable) Annotate(a infer.Annotator) {
	a.SetToken("index", "ProjectSharedVariable")
	a.Describe(&r, "A shared variable for a project on Coolify v4.3.0+. Adopts an existing key in this scope and updates declared settings in place. Delete removes the variable. Import using project/<projectUuid>/<id>; URL-encode path components containing slashes.")
}

func (args *ProjectSharedVariableArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.ProjectUUID, "UUID of the owning project. Changing it replaces the variable.")
}

func (args ProjectSharedVariableArgs) sharedFields() SharedVariableArgs {
	return args.SharedVariableArgs
}

func (args ProjectSharedVariableArgs) sharedScope() coolify.SharedVariableScope {
	return coolify.SharedVariableScope{Type: "project", ProjectUUID: args.ProjectUUID}
}

func (args ProjectSharedVariableArgs) withSharedFields(fields SharedVariableArgs, scope coolify.SharedVariableScope) ProjectSharedVariableArgs {
	args.SharedVariableArgs = fields
	args.ProjectUUID = scope.ProjectUUID
	return args
}

func projectSharedVariableState(args ProjectSharedVariableArgs, id int) ProjectSharedVariableState {
	return ProjectSharedVariableState{ProjectSharedVariableArgs: args, SharedVariableOutputs: sharedVariableOutputs(args.sharedScope(), args.Key, id)}
}

func (ProjectSharedVariable) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[ProjectSharedVariableArgs], error) {
	return checkSharedVariable[ProjectSharedVariableArgs](ctx, req)
}

func (ProjectSharedVariable) Create(ctx context.Context, req infer.CreateRequest[ProjectSharedVariableArgs]) (infer.CreateResponse[ProjectSharedVariableState], error) {
	return createSharedVariable(ctx, req, projectSharedVariableState)
}

func (ProjectSharedVariable) Diff(ctx context.Context, req infer.DiffRequest[ProjectSharedVariableArgs, ProjectSharedVariableState]) (infer.DiffResponse, error) {
	return diffSharedVariable(req.State.ProjectSharedVariableArgs, req.Inputs), nil
}

func (ProjectSharedVariable) Update(ctx context.Context, req infer.UpdateRequest[ProjectSharedVariableArgs, ProjectSharedVariableState]) (infer.UpdateResponse[ProjectSharedVariableState], error) {
	return updateSharedVariable(ctx, req, req.State.ProjectSharedVariableArgs, projectSharedVariableState)
}

func (ProjectSharedVariable) Read(ctx context.Context, req infer.ReadRequest[ProjectSharedVariableArgs, ProjectSharedVariableState]) (infer.ReadResponse[ProjectSharedVariableArgs, ProjectSharedVariableState], error) {
	return readSharedVariable(ctx, req, req.State.ProjectSharedVariableArgs, req.State.VariableID == 0, projectSharedVariableState)
}

func (ProjectSharedVariable) Delete(ctx context.Context, req infer.DeleteRequest[ProjectSharedVariableState]) (infer.DeleteResponse, error) {
	return deleteSharedVariable(ctx, "project", req.ID)
}

func (ProjectSharedVariable) WireDependencies(f infer.FieldSelector, args *ProjectSharedVariableArgs, state *ProjectSharedVariableState) {
	f.OutputField(&state.Reference).DependsOn(f.InputField(&args.Key))
	f.OutputField(&state.VariableID).NeverSecret()
}

// EnvironmentSharedVariable manages one shared variable for a project environment.
type EnvironmentSharedVariable struct{}

type EnvironmentSharedVariableArgs struct {
	SharedVariableArgs
	ProjectUUID     string `pulumi:"projectUuid"`
	EnvironmentName string `pulumi:"environmentName"`
}

type EnvironmentSharedVariableState struct {
	EnvironmentSharedVariableArgs
	SharedVariableOutputs
}

func (r *EnvironmentSharedVariable) Annotate(a infer.Annotator) {
	a.SetToken("index", "EnvironmentSharedVariable")
	a.Describe(&r, "A shared variable for a project environment on Coolify v4.3.0+. Adopts an existing key in this scope and updates declared settings in place. Delete removes the variable. Import using environment/<projectUuid>/<environmentName>/<id>; URL-encode path components containing slashes.")
}

func (args *EnvironmentSharedVariableArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.ProjectUUID, "UUID of the owning project. Changing it replaces the variable.")
	a.Describe(&args.EnvironmentName, "Name or UUID of the owning project environment. Changing it replaces the variable.")
}

func (args EnvironmentSharedVariableArgs) sharedFields() SharedVariableArgs {
	return args.SharedVariableArgs
}

func (args EnvironmentSharedVariableArgs) sharedScope() coolify.SharedVariableScope {
	return coolify.SharedVariableScope{Type: "environment", ProjectUUID: args.ProjectUUID, EnvironmentName: args.EnvironmentName}
}

func (args EnvironmentSharedVariableArgs) withSharedFields(fields SharedVariableArgs, scope coolify.SharedVariableScope) EnvironmentSharedVariableArgs {
	args.SharedVariableArgs = fields
	args.ProjectUUID = scope.ProjectUUID
	args.EnvironmentName = scope.EnvironmentName
	return args
}

func environmentSharedVariableState(args EnvironmentSharedVariableArgs, id int) EnvironmentSharedVariableState {
	return EnvironmentSharedVariableState{EnvironmentSharedVariableArgs: args, SharedVariableOutputs: sharedVariableOutputs(args.sharedScope(), args.Key, id)}
}

func (EnvironmentSharedVariable) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[EnvironmentSharedVariableArgs], error) {
	return checkSharedVariable[EnvironmentSharedVariableArgs](ctx, req)
}

func (EnvironmentSharedVariable) Create(ctx context.Context, req infer.CreateRequest[EnvironmentSharedVariableArgs]) (infer.CreateResponse[EnvironmentSharedVariableState], error) {
	return createSharedVariable(ctx, req, environmentSharedVariableState)
}

func (EnvironmentSharedVariable) Diff(ctx context.Context, req infer.DiffRequest[EnvironmentSharedVariableArgs, EnvironmentSharedVariableState]) (infer.DiffResponse, error) {
	return diffSharedVariable(req.State.EnvironmentSharedVariableArgs, req.Inputs), nil
}

func (EnvironmentSharedVariable) Update(ctx context.Context, req infer.UpdateRequest[EnvironmentSharedVariableArgs, EnvironmentSharedVariableState]) (infer.UpdateResponse[EnvironmentSharedVariableState], error) {
	return updateSharedVariable(ctx, req, req.State.EnvironmentSharedVariableArgs, environmentSharedVariableState)
}

func (EnvironmentSharedVariable) Read(ctx context.Context, req infer.ReadRequest[EnvironmentSharedVariableArgs, EnvironmentSharedVariableState]) (infer.ReadResponse[EnvironmentSharedVariableArgs, EnvironmentSharedVariableState], error) {
	return readSharedVariable(ctx, req, req.State.EnvironmentSharedVariableArgs, req.State.VariableID == 0, environmentSharedVariableState)
}

func (EnvironmentSharedVariable) Delete(ctx context.Context, req infer.DeleteRequest[EnvironmentSharedVariableState]) (infer.DeleteResponse, error) {
	return deleteSharedVariable(ctx, "environment", req.ID)
}

func (EnvironmentSharedVariable) WireDependencies(f infer.FieldSelector, args *EnvironmentSharedVariableArgs, state *EnvironmentSharedVariableState) {
	f.OutputField(&state.Reference).DependsOn(f.InputField(&args.Key))
	f.OutputField(&state.VariableID).NeverSecret()
}

// ServerSharedVariable manages one shared variable for a server.
type ServerSharedVariable struct{}

type ServerSharedVariableArgs struct {
	SharedVariableArgs
	ServerUUID string `pulumi:"serverUuid"`
}

type ServerSharedVariableState struct {
	ServerSharedVariableArgs
	SharedVariableOutputs
}

func (r *ServerSharedVariable) Annotate(a infer.Annotator) {
	a.SetToken("index", "ServerSharedVariable")
	a.Describe(&r, "A shared variable for a server on Coolify v4.3.0+. Adopts an existing key in this scope and updates declared settings in place. Delete removes the variable. Import using server/<serverUuid>/<id>; URL-encode path components containing slashes.")
}

func (args *ServerSharedVariableArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.ServerUUID, "UUID of the owning server. Changing it replaces the variable.")
}

func (args ServerSharedVariableArgs) sharedFields() SharedVariableArgs {
	return args.SharedVariableArgs
}

func (args ServerSharedVariableArgs) sharedScope() coolify.SharedVariableScope {
	return coolify.SharedVariableScope{Type: "server", ServerUUID: args.ServerUUID}
}

func (args ServerSharedVariableArgs) withSharedFields(fields SharedVariableArgs, scope coolify.SharedVariableScope) ServerSharedVariableArgs {
	args.SharedVariableArgs = fields
	args.ServerUUID = scope.ServerUUID
	return args
}

func serverSharedVariableState(args ServerSharedVariableArgs, id int) ServerSharedVariableState {
	return ServerSharedVariableState{ServerSharedVariableArgs: args, SharedVariableOutputs: sharedVariableOutputs(args.sharedScope(), args.Key, id)}
}

func (ServerSharedVariable) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[ServerSharedVariableArgs], error) {
	return checkSharedVariable[ServerSharedVariableArgs](ctx, req)
}

func (ServerSharedVariable) Create(ctx context.Context, req infer.CreateRequest[ServerSharedVariableArgs]) (infer.CreateResponse[ServerSharedVariableState], error) {
	return createSharedVariable(ctx, req, serverSharedVariableState)
}

func (ServerSharedVariable) Diff(ctx context.Context, req infer.DiffRequest[ServerSharedVariableArgs, ServerSharedVariableState]) (infer.DiffResponse, error) {
	return diffSharedVariable(req.State.ServerSharedVariableArgs, req.Inputs), nil
}

func (ServerSharedVariable) Update(ctx context.Context, req infer.UpdateRequest[ServerSharedVariableArgs, ServerSharedVariableState]) (infer.UpdateResponse[ServerSharedVariableState], error) {
	return updateSharedVariable(ctx, req, req.State.ServerSharedVariableArgs, serverSharedVariableState)
}

func (ServerSharedVariable) Read(ctx context.Context, req infer.ReadRequest[ServerSharedVariableArgs, ServerSharedVariableState]) (infer.ReadResponse[ServerSharedVariableArgs, ServerSharedVariableState], error) {
	return readSharedVariable(ctx, req, req.State.ServerSharedVariableArgs, req.State.VariableID == 0, serverSharedVariableState)
}

func (ServerSharedVariable) Delete(ctx context.Context, req infer.DeleteRequest[ServerSharedVariableState]) (infer.DeleteResponse, error) {
	return deleteSharedVariable(ctx, "server", req.ID)
}

func (ServerSharedVariable) WireDependencies(f infer.FieldSelector, args *ServerSharedVariableArgs, state *ServerSharedVariableState) {
	f.OutputField(&state.Reference).DependsOn(f.InputField(&args.Key))
	f.OutputField(&state.VariableID).NeverSecret()
}
