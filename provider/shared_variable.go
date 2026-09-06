package provider

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

// SharedVariableArgs contains the fields common to all four shared variable scopes.
// Pointer inputs distinguish unmanaged settings from explicit empty/false values.
type SharedVariableArgs struct {
	Key         string  `pulumi:"key"`
	Value       *string `pulumi:"value,optional" provider:"secret"`
	IsLiteral   *bool   `pulumi:"isLiteral,optional"`
	IsMultiline *bool   `pulumi:"isMultiline,optional"`
	IsShownOnce *bool   `pulumi:"isShownOnce,optional"`
	Comment     *string `pulumi:"comment,optional"`
}

func (args *SharedVariableArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Key, "Variable key. An existing variable with this key in the same scope is adopted. Renaming updates it in place.")
	a.Describe(&args.Value, "Variable value, stored as a Pulumi secret. Omit to leave it unmanaged; an empty string clears it. Refresh preserves the previous value when Coolify omits it. Reading values requires read:sensitive or root permission.")
	a.Describe(&args.IsLiteral, "Treat the value literally. Omit to preserve the current setting; false explicitly disables it.")
	a.Describe(&args.IsMultiline, "Treat the value as multiline. Omit to preserve the current setting; false explicitly disables it.")
	a.Describe(&args.IsShownOnce, "Hide the value from subsequent API reads, even with sensitive-read permission. The API still allows updates. Omit to preserve the current setting.")
	a.Describe(&args.Comment, "Optional comment (maximum 256 characters). Omit to leave it unmanaged; an empty string clears it.")
}

type SharedVariableOutputs struct {
	VariableID int    `pulumi:"variableId"`
	Reference  string `pulumi:"reference"`
}

func (state *SharedVariableOutputs) Annotate(a infer.Annotator) {
	a.Describe(&state.VariableID, "Numeric ID of the shared variable in Coolify.")
	a.Describe(&state.Reference, "Coolify reference such as {{team.KEY}}. Use as a resource environment variable value; the output also establishes a Pulumi dependency. Applying shared variables to running workloads requires redeployment.")
}

type sharedVariableInputs[A any] interface {
	sharedFields() SharedVariableArgs
	sharedScope() coolify.SharedVariableScope
	withSharedFields(SharedVariableArgs, coolify.SharedVariableScope) A
}

var sharedVariableKeyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

func checkSharedVariable[A sharedVariableInputs[A]](ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[A], error) {
	args, failures, err := infer.DefaultCheck[A](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[A]{}, err
	}
	fields := args.sharedFields()
	if value := req.NewInputs.Get("key"); value.IsString() {
		fields.Key = strings.TrimSpace(fields.Key)
		if len(fields.Key) > 255 || !sharedVariableKeyPattern.MatchString(fields.Key) {
			failures = append(failures, p.CheckFailure{Property: "key", Reason: "key must start with a letter or underscore, contain only letters, numbers, underscores or dots, and be at most 255 characters"})
		}
	}
	if value := req.NewInputs.Get("comment"); value.IsString() && utf8.RuneCountInString(value.AsString()) > 256 {
		failures = append(failures, p.CheckFailure{Property: "comment", Reason: "comment must be at most 256 characters"})
	}
	for _, name := range []string{"projectUuid", "environmentName", "serverUuid"} {
		if value := req.NewInputs.Get(name); value.IsString() && strings.TrimSpace(value.AsString()) == "" {
			failures = append(failures, p.CheckFailure{Property: name, Reason: name + " must not be empty"})
		}
	}
	return infer.CheckResponse[A]{Inputs: args.withSharedFields(fields, args.sharedScope()), Failures: failures}, nil
}

func sharedVariableID(scope coolify.SharedVariableScope, id int) string {
	parts := []string{scope.Type}
	switch scope.Type {
	case "project":
		parts = append(parts, url.PathEscape(scope.ProjectUUID))
	case "environment":
		parts = append(parts, url.PathEscape(scope.ProjectUUID), url.PathEscape(scope.EnvironmentName))
	case "server":
		parts = append(parts, url.PathEscape(scope.ServerUUID))
	}
	return strings.Join(append(parts, strconv.Itoa(id)), "/")
}

func parseSharedVariableID(kind, id string) (coolify.SharedVariableScope, int, error) {
	scope := coolify.SharedVariableScope{Type: kind}
	parts := strings.Split(id, "/")
	want := map[string]int{"team": 2, "project": 3, "environment": 4, "server": 3}[kind]
	invalid := func() (coolify.SharedVariableScope, int, error) {
		return scope, 0, fmt.Errorf("invalid %s shared variable import ID %q; expected %s/<id> with the project/server UUID and environment name between scope and ID where applicable", kind, id, kind)
	}
	if want == 0 || len(parts) != want || parts[0] != kind {
		return invalid()
	}
	number, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil || number <= 0 || strconv.Itoa(number) != parts[len(parts)-1] {
		return invalid()
	}
	for i := 1; i < len(parts)-1; i++ {
		parts[i], err = url.PathUnescape(parts[i])
		if err != nil || strings.TrimSpace(parts[i]) == "" {
			return invalid()
		}
	}
	switch kind {
	case "project":
		scope.ProjectUUID = parts[1]
	case "environment":
		scope.ProjectUUID, scope.EnvironmentName = parts[1], parts[2]
	case "server":
		scope.ServerUUID = parts[1]
	}
	return scope, number, nil
}

func sharedVariableOutputs(scope coolify.SharedVariableScope, key string, id int) SharedVariableOutputs {
	return SharedVariableOutputs{VariableID: id, Reference: "{{" + scope.Type + "." + key + "}}"}
}

// A missing list endpoint must not be mistaken for an absent variable. For
// scoped resources verify that the owner is gone before dropping the state.
func listSharedVariables(ctx context.Context, scope coolify.SharedVariableScope) ([]coolify.SharedVariable, error) {
	c := client(ctx)
	variables, err := c.ListSharedVariables(ctx, scope)
	if !coolify.IsNotFound(err) {
		return variables, err
	}
	var ownerErr error
	switch scope.Type {
	case "project":
		_, ownerErr = c.GetProject(ctx, scope.ProjectUUID)
	case "environment":
		_, ownerErr = c.GetEnvironment(ctx, scope.ProjectUUID, scope.EnvironmentName)
	case "server":
		_, ownerErr = c.GetServer(ctx, scope.ServerUUID)
	}
	if coolify.IsNotFound(ownerErr) {
		return nil, nil
	}
	if ownerErr != nil {
		return nil, ownerErr
	}
	return nil, fmt.Errorf("reading shared variables requires Coolify v4.3.0 or newer: %w", err)
}

func findSharedVariable(ctx context.Context, scope coolify.SharedVariableScope, id int) (*coolify.SharedVariable, error) {
	variables, err := listSharedVariables(ctx, scope)
	if err != nil {
		return nil, err
	}
	for _, variable := range variables {
		if variable.ID == id {
			return &variable, nil
		}
	}
	return nil, nil
}

func createSharedVariable[A sharedVariableInputs[A], S any](ctx context.Context, req infer.CreateRequest[A], state func(A, int) S) (infer.CreateResponse[S], error) {
	if req.DryRun {
		return infer.CreateResponse[S]{Output: state(req.Inputs, 0)}, nil
	}
	scope, desired := req.Inputs.sharedScope(), req.Inputs.sharedFields()
	variables, err := listSharedVariables(ctx, scope)
	if err != nil {
		return infer.CreateResponse[S]{}, err
	}
	adopt := func(current coolify.SharedVariable) (infer.CreateResponse[S], error) {
		if current.ID <= 0 {
			return infer.CreateResponse[S]{}, fmt.Errorf("coolify returned an invalid shared variable ID")
		}
		if err := applySharedVariable(ctx, scope, current, SharedVariableArgs{}, desired); err != nil {
			return infer.CreateResponse[S]{}, err
		}
		return infer.CreateResponse[S]{ID: sharedVariableID(scope, current.ID), Output: state(req.Inputs, current.ID)}, nil
	}
	for _, variable := range variables {
		if variable.Key == desired.Key {
			return adopt(variable)
		}
	}
	id, err := client(ctx).CreateSharedVariable(ctx, scope, coolify.SharedVariableInput{
		Key: &desired.Key, Value: desired.Value, IsLiteral: desired.IsLiteral,
		IsMultiline: desired.IsMultiline, IsShownOnce: desired.IsShownOnce, Comment: desired.Comment,
	})
	if coolify.IsConflict(err) {
		// Another writer (or a retried POST) may have created the key after our list.
		variables, listErr := listSharedVariables(ctx, scope)
		if listErr != nil {
			return infer.CreateResponse[S]{}, listErr
		}
		for _, variable := range variables {
			if variable.Key == desired.Key {
				return adopt(variable)
			}
		}
	}
	if err != nil {
		return infer.CreateResponse[S]{}, err
	}
	return infer.CreateResponse[S]{ID: sharedVariableID(scope, id), Output: state(req.Inputs, id)}, nil
}

func diffSharedVariable[A sharedVariableInputs[A]](old, next A) infer.DiffResponse {
	diff := diffArgs(old.sharedFields(), next.sharedFields())
	for key, change := range diffArgs(old, next, "projectUuid", "environmentName", "serverUuid") {
		diff[key] = change
	}
	// Aliases (environment name vs UUID) can address the same variable. Delete
	// first when the key stays the same so Create never adopts the old record.
	return diffResponse(diff, old.sharedFields().Key == next.sharedFields().Key)
}

func updateSharedVariable[A sharedVariableInputs[A], S any](ctx context.Context, req infer.UpdateRequest[A, S], previous A, state func(A, int) S) (infer.UpdateResponse[S], error) {
	scope, id, err := parseSharedVariableID(req.Inputs.sharedScope().Type, req.ID)
	if err != nil {
		return infer.UpdateResponse[S]{}, err
	}
	if req.DryRun {
		return infer.UpdateResponse[S]{Output: state(req.Inputs, id)}, nil
	}
	if scope != req.Inputs.sharedScope() {
		return infer.UpdateResponse[S]{}, fmt.Errorf("changing a shared variable scope requires replacement")
	}
	current, err := findSharedVariable(ctx, scope, id)
	if err != nil {
		return infer.UpdateResponse[S]{}, err
	}
	if current == nil {
		return infer.UpdateResponse[S]{}, fmt.Errorf("shared variable %q no longer exists; refresh before updating", req.ID)
	}
	if err := applySharedVariable(ctx, scope, *current, previous.sharedFields(), req.Inputs.sharedFields()); err != nil {
		return infer.UpdateResponse[S]{}, err
	}
	return infer.UpdateResponse[S]{Output: state(req.Inputs, id)}, nil
}

func applySharedVariable(ctx context.Context, scope coolify.SharedVariableScope, current coolify.SharedVariable, previous, desired SharedVariableArgs) error {
	var body coolify.SharedVariableInput
	var patch patch
	patch.text(&body.Key, desired.Key, current.Key)
	value := previous.Value
	if current.ValuePresent {
		value = coolify.Ptr(coolify.Deref(current.Value))
	}
	if desired.Value != nil && (value == nil || *desired.Value != *value) {
		body.Value, patch.changed = desired.Value, true
	}
	if desired.Comment != nil {
		patch.text(&body.Comment, *desired.Comment, coolify.Deref(current.Comment))
	}
	if desired.IsLiteral != nil {
		patch.boolean(&body.IsLiteral, *desired.IsLiteral, current.IsLiteral)
	}
	if desired.IsMultiline != nil {
		patch.boolean(&body.IsMultiline, *desired.IsMultiline, current.IsMultiline)
	}
	if desired.IsShownOnce != nil {
		patch.boolean(&body.IsShownOnce, *desired.IsShownOnce, current.IsShownOnce)
	}
	if !patch.changed {
		return nil
	}
	_, err := client(ctx).UpdateSharedVariable(ctx, scope, current.ID, body)
	return err
}

func readSharedVariable[A sharedVariableInputs[A], S any](ctx context.Context, req infer.ReadRequest[A, S], previous A, importing bool, state func(A, int) S) (infer.ReadResponse[A, S], error) {
	scope, id, err := parseSharedVariableID(req.Inputs.sharedScope().Type, req.ID)
	if err != nil {
		return infer.ReadResponse[A, S]{}, err
	}
	current, err := findSharedVariable(ctx, scope, id)
	if err != nil {
		return infer.ReadResponse[A, S]{}, err
	}
	if current == nil {
		return infer.ReadResponse[A, S]{}, nil
	}
	inputs := req.Inputs
	if !importing && inputs.sharedFields().Key == "" {
		// Some refresh callers supply only the prior state, without inputs.
		inputs = previous
	}
	fields := inputs.sharedFields()
	fields.Key = current.Key
	if (importing || fields.Value != nil) && current.ValuePresent {
		fields.Value = coolify.Ptr(coolify.Deref(current.Value))
	}
	if importing || fields.Comment != nil {
		fields.Comment = coolify.Ptr(coolify.Deref(current.Comment))
	}
	if importing || fields.IsLiteral != nil {
		fields.IsLiteral = coolify.Ptr(current.IsLiteral)
	}
	if importing || fields.IsMultiline != nil {
		fields.IsMultiline = coolify.Ptr(current.IsMultiline)
	}
	if importing || fields.IsShownOnce != nil {
		fields.IsShownOnce = coolify.Ptr(current.IsShownOnce)
	}
	inputs = inputs.withSharedFields(fields, scope)
	return infer.ReadResponse[A, S]{ID: req.ID, Inputs: inputs, State: state(inputs, id)}, nil
}

func deleteSharedVariable(ctx context.Context, kind, id string) (infer.DeleteResponse, error) {
	scope, number, err := parseSharedVariableID(kind, id)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	current, err := findSharedVariable(ctx, scope, number)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	if current == nil {
		return infer.DeleteResponse{}, nil
	}
	err = client(ctx).DeleteSharedVariable(ctx, scope, number)
	if err != nil && !coolify.IsNotFound(err) {
		return infer.DeleteResponse{}, err
	}
	return infer.DeleteResponse{}, nil
}
