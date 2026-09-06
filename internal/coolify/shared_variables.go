package coolify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// SharedVariableScope identifies the owner of a shared environment variable.
// Team scope uses the current API token's team. EnvironmentName accepts either
// an environment name or UUID within ProjectUUID.
type SharedVariableScope struct {
	Type            string
	ProjectUUID     string
	EnvironmentName string
	ServerUUID      string
}

func (s SharedVariableScope) validate() error {
	switch s.Type {
	case "team":
	case "project":
		if strings.TrimSpace(s.ProjectUUID) == "" {
			return fmt.Errorf("coolify: project shared variables require a project UUID")
		}
	case "environment":
		if strings.TrimSpace(s.ProjectUUID) == "" || strings.TrimSpace(s.EnvironmentName) == "" {
			return fmt.Errorf("coolify: environment shared variables require a project UUID and environment name or UUID")
		}
	case "server":
		if strings.TrimSpace(s.ServerUUID) == "" {
			return fmt.Errorf("coolify: server shared variables require a server UUID")
		}
	default:
		return fmt.Errorf("coolify: unsupported shared variable scope %q", s.Type)
	}
	return nil
}

// SharedVariable is a shared environment variable returned by Coolify. A value
// can be omitted when the token cannot read sensitive data or is_shown_once is
// enabled. ValuePresent distinguishes that omission from an explicit null.
type SharedVariable struct {
	ID           int     `json:"id"`
	Key          string  `json:"key"`
	Value        *string `json:"value"`
	ValuePresent bool    `json:"-"`
	IsLiteral    bool    `json:"is_literal"`
	IsMultiline  bool    `json:"is_multiline"`
	IsShownOnce  bool    `json:"is_shown_once"`
	Comment      *string `json:"comment"`
}

// UnmarshalJSON tracks whether the response includes value, even when null.
func (v *SharedVariable) UnmarshalJSON(data []byte) error {
	type variable SharedVariable
	var decoded variable
	wire := struct {
		*variable
		IsLiteral   sharedVariableBool `json:"is_literal"`
		IsMultiline sharedVariableBool `json:"is_multiline"`
		IsShownOnce sharedVariableBool `json:"is_shown_once"`
	}{variable: &decoded}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}
	_, decoded.ValuePresent = fields["value"]
	decoded.IsLiteral = bool(wire.IsLiteral)
	decoded.IsMultiline = bool(wire.IsMultiline)
	decoded.IsShownOnce = bool(wire.IsShownOnce)
	*v = SharedVariable(decoded)
	return nil
}

// Coolify's model does not cast these database flags to JSON booleans.
type sharedVariableBool bool

func (b *sharedVariableBool) UnmarshalJSON(data []byte) error {
	switch string(bytes.TrimSpace(data)) {
	case "true", "1", `"1"`:
		*b = true
	case "false", "0", `"0"`, "null":
		*b = false
	default:
		return fmt.Errorf("coolify API: shared variable flag must be a boolean or 0/1")
	}
	return nil
}

// SharedVariableInput includes only fields to create or update. Pointers retain
// explicit empty strings and false flags while nil fields are omitted.
type SharedVariableInput struct {
	Key         *string `json:"key,omitempty"`
	Value       *string `json:"value,omitempty"`
	IsLiteral   *bool   `json:"is_literal,omitempty"`
	IsMultiline *bool   `json:"is_multiline,omitempty"`
	IsShownOnce *bool   `json:"is_shown_once,omitempty"`
	Comment     *string `json:"comment,omitempty"`
}

// ListSharedVariables lists variables belonging to a single scope.
func (c *Client) ListSharedVariables(ctx context.Context, scope SharedVariableScope) ([]SharedVariable, error) {
	if err := scope.validate(); err != nil {
		return nil, err
	}
	switch scope.Type {
	case "team":
		return decode[[]SharedVariable](c.api.ListTeamSharedEnvs(ctx))
	case "project":
		return decode[[]SharedVariable](c.api.ListProjectSharedEnvs(ctx, scope.ProjectUUID))
	case "environment":
		return decode[[]SharedVariable](c.api.ListEnvironmentSharedEnvs(ctx, scope.ProjectUUID, scope.EnvironmentName))
	default: // validated server scope
		return decode[[]SharedVariable](c.api.ListServerSharedEnvs(ctx, scope.ServerUUID))
	}
}

// CreateSharedVariable creates one variable and returns its numeric API ID.
func (c *Client) CreateSharedVariable(ctx context.Context, scope SharedVariableScope, input SharedVariableInput) (int, error) {
	if err := scope.validate(); err != nil {
		return 0, err
	}
	body, err := json.Marshal(input)
	if err != nil {
		return 0, fmt.Errorf("coolify: encode shared variable: %w", err)
	}
	var resp *http.Response
	switch scope.Type {
	case "team":
		resp, err = c.api.CreateTeamSharedEnvWithBody(ctx, "application/json", bytes.NewReader(body))
	case "project":
		resp, err = c.api.CreateProjectSharedEnvWithBody(ctx, scope.ProjectUUID, "application/json", bytes.NewReader(body))
	case "environment":
		resp, err = c.api.CreateEnvironmentSharedEnvWithBody(ctx, scope.ProjectUUID, scope.EnvironmentName, "application/json", bytes.NewReader(body))
	case "server":
		resp, err = c.api.CreateServerSharedEnvWithBody(ctx, scope.ServerUUID, "application/json", bytes.NewReader(body))
	}
	out, err := decode[struct {
		ID int `json:"id"`
	}](resp, err)
	if err != nil {
		return 0, err
	}
	if out.ID <= 0 {
		return 0, fmt.Errorf("coolify API: create shared variable response is missing a positive integer id")
	}
	return out.ID, nil
}

// UpdateSharedVariable applies the supplied fields to a variable in scope.
func (c *Client) UpdateSharedVariable(ctx context.Context, scope SharedVariableScope, id int, input SharedVariableInput) (SharedVariable, error) {
	if err := scope.validate(); err != nil {
		return SharedVariable{}, err
	}
	if id <= 0 {
		return SharedVariable{}, fmt.Errorf("coolify: shared variable id must be positive")
	}
	body, err := json.Marshal(input)
	if err != nil {
		return SharedVariable{}, fmt.Errorf("coolify: encode shared variable: %w", err)
	}
	switch scope.Type {
	case "team":
		return decode[SharedVariable](c.api.UpdateTeamSharedEnvWithBody(ctx, id, "application/json", bytes.NewReader(body)))
	case "project":
		return decode[SharedVariable](c.api.UpdateProjectSharedEnvWithBody(ctx, scope.ProjectUUID, id, "application/json", bytes.NewReader(body)))
	case "environment":
		return decode[SharedVariable](c.api.UpdateEnvironmentSharedEnvWithBody(ctx, scope.ProjectUUID, scope.EnvironmentName, id, "application/json", bytes.NewReader(body)))
	default: // validated server scope
		return decode[SharedVariable](c.api.UpdateServerSharedEnvWithBody(ctx, scope.ServerUUID, id, "application/json", bytes.NewReader(body)))
	}
}

// DeleteSharedVariable deletes a variable by its numeric ID within scope.
func (c *Client) DeleteSharedVariable(ctx context.Context, scope SharedVariableScope, id int) error {
	if err := scope.validate(); err != nil {
		return err
	}
	if id <= 0 {
		return fmt.Errorf("coolify: shared variable id must be positive")
	}
	switch scope.Type {
	case "team":
		return check(c.api.DeleteTeamSharedEnv(ctx, id))
	case "project":
		return check(c.api.DeleteProjectSharedEnv(ctx, scope.ProjectUUID, id))
	case "environment":
		return check(c.api.DeleteEnvironmentSharedEnv(ctx, scope.ProjectUUID, scope.EnvironmentName, id))
	default: // validated server scope
		return check(c.api.DeleteServerSharedEnv(ctx, scope.ServerUUID, id))
	}
}
