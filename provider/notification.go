package provider

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
	"github.com/pulumi/pulumi/sdk/v3/go/property"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify"
)

// infer v1.6.0's secret annotation walker turns already-secret computed
// values into known zero values during Check. Preserve the input unknowns for
// notification resources so a preview cannot turn a future credential into a
// request to clear it. Keep inferred validation and secret annotations intact.
func checkNotificationUnknowns(check func(context.Context, p.CheckRequest) (p.CheckResponse, error)) func(context.Context, p.CheckRequest) (p.CheckResponse, error) {
	return func(ctx context.Context, req p.CheckRequest) (p.CheckResponse, error) {
		response, err := check(ctx, req)
		if err != nil || !strings.HasPrefix(string(req.Urn.Type()), "coolify:index:Notification") {
			return response, err
		}
		response.Inputs = restoreNotificationUnknowns(property.New(req.Inputs), property.New(response.Inputs)).AsMap()
		return response, nil
	}
}

func restoreNotificationUnknowns(original, checked property.Value) property.Value {
	if original.IsComputed() {
		return original.WithSecret(original.Secret() || checked.Secret())
	}
	if original.IsMap() && checked.IsMap() {
		out := checked.AsMap()
		for key, value := range original.AsMap().All {
			out = out.Set(key, restoreNotificationUnknowns(value, out.Get(key)))
		}
		return property.New(out).WithSecret(checked.Secret())
	}
	return checked
}

// Notification endpoints address a team singleton, not a record by UUID. Keep
// the team in the Pulumi ID and verify it before writing through a changed token.
func notificationIdentity(settings coolify.NotificationSettings, channel, expected string) (int, error) {
	team, ok := settings["team_id"].(float64)
	if !ok || team < 1 || team != math.Trunc(team) || team > float64(1<<53-1) {
		return 0, fmt.Errorf("coolify %s notification settings returned an invalid team_id", channel)
	}
	id := strconv.Itoa(int(team)) + "/" + channel
	if expected != "" && expected != id {
		return 0, fmt.Errorf("coolify notification identity mismatch: resource %q does not match API token team %q", expected, id)
	}
	return int(team), nil
}

func getNotification(ctx context.Context, channel, id string) (coolify.NotificationSettings, int, error) {
	settings, err := client(ctx).GetNotificationSettings(ctx, channel)
	if err != nil {
		// A 404 means the endpoint is unavailable, not that a singleton was
		// deleted. Do not drop the resource from state on unsupported servers.
		return nil, 0, fmt.Errorf("reading %s notifications (requires Coolify v4.3.0+): %w", channel, err)
	}
	teamID, err := notificationIdentity(settings, channel, id)
	return settings, teamID, err
}

func createNotification[A, S any](ctx context.Context, channel string, req infer.CreateRequest[A], state func(A, int) S) (infer.CreateResponse[S], error) {
	if req.DryRun {
		return infer.CreateResponse[S]{Output: state(req.Inputs, 0)}, nil
	}
	current, teamID, err := getNotification(ctx, channel, "")
	if err != nil {
		return infer.CreateResponse[S]{}, err
	}
	var previous A
	if err := applyNotification(ctx, channel, current, previous, req.Inputs); err != nil {
		return infer.CreateResponse[S]{}, err
	}
	return infer.CreateResponse[S]{ID: strconv.Itoa(teamID) + "/" + channel, Output: state(req.Inputs, teamID)}, nil
}

func updateNotification[A, S any](ctx context.Context, channel string, req infer.UpdateRequest[A, S], previous A, teamID int, state func(A, int) S) (infer.UpdateResponse[S], error) {
	if req.DryRun {
		return infer.UpdateResponse[S]{Output: state(req.Inputs, teamID)}, nil
	}
	current, teamID, err := getNotification(ctx, channel, req.ID)
	if err != nil {
		return infer.UpdateResponse[S]{}, err
	}
	if err := applyNotification(ctx, channel, current, previous, req.Inputs); err != nil {
		return infer.UpdateResponse[S]{}, err
	}
	return infer.UpdateResponse[S]{Output: state(req.Inputs, teamID)}, nil
}

func readNotification[A, S any](ctx context.Context, channel string, req infer.ReadRequest[A, S], importing bool, state func(A, int) S) (infer.ReadResponse[A, S], error) {
	current, teamID, err := getNotification(ctx, channel, req.ID)
	if err != nil {
		return infer.ReadResponse[A, S]{}, err
	}
	inputs := req.Inputs
	for _, field := range notificationFields(&inputs, channel, importing) {
		if field.value.IsNil() && !importing {
			continue
		}
		value, readable, err := field.read(current)
		if err != nil {
			return infer.ReadResponse[A, S]{}, err
		}
		if !readable {
			// Omitted/redacted secrets retain the declared value. In particular,
			// import never turns a masking placeholder into a real credential.
			continue
		}
		if value == nil {
			field.value.SetZero()
			continue
		}
		ptr := reflect.New(field.value.Type().Elem())
		ptr.Elem().Set(reflect.ValueOf(value).Convert(ptr.Elem().Type()))
		field.value.Set(ptr)
	}
	return infer.ReadResponse[A, S]{ID: req.ID, Inputs: inputs, State: state(inputs, teamID)}, nil
}

func deleteNotification(ctx context.Context, channel, id string) (infer.DeleteResponse, error) {
	current, _, err := getNotification(ctx, channel, id)
	if err != nil {
		return infer.DeleteResponse{}, err
	}
	keys := []string{channel + "_enabled"}
	if channel == "email" {
		keys = []string{"smtp_enabled", "resend_enabled", "use_instance_email_settings"}
	}
	patch := coolify.NotificationSettings{}
	for _, key := range keys {
		if enabled, ok := current[key].(bool); !ok || enabled {
			patch[key] = false
		}
	}
	if len(patch) > 0 {
		if _, err := client(ctx).UpdateNotificationSettings(ctx, channel, patch); err != nil {
			return infer.DeleteResponse{}, err
		}
	}
	return infer.DeleteResponse{}, nil
}

// The API's channel-specific fields and repeated event suffixes are mapped
// from our typed argument structs in one place. Reflection is limited to these
// pointer scalar fields; their tags also define the public Pulumi schema.
type notificationField struct {
	key    string
	value  reflect.Value
	secret bool
}

func notificationFields[A any](args *A, channel string, importing bool) []notificationField {
	root := reflect.ValueOf(args).Elem()
	var fields []notificationField
	for i := 0; i < root.NumField(); i++ {
		typ, value := root.Type().Field(i), root.Field(i)
		if typ.Name == "Events" || typ.Name == "Threads" {
			if value.IsNil() && !importing {
				continue
			}
			// Clone the group so Read cannot mutate pointers held in prior state.
			group := reflect.New(value.Type().Elem())
			if !value.IsNil() {
				group.Elem().Set(value.Elem())
			}
			value.Set(group)
			for j := 0; j < group.Elem().NumField(); j++ {
				event := strings.Split(group.Elem().Type().Field(j).Tag.Get("json"), ",")[0]
				key := event + "_" + channel + "_notifications"
				if typ.Name == "Threads" {
					key = "telegram_notifications_" + event + "_thread_id"
				}
				fields = append(fields, notificationField{
					key: key, value: group.Elem().Field(j),
					secret: group.Elem().Type().Field(j).Tag.Get("provider") == "secret",
				})
			}
			continue
		}
		fields = append(fields, notificationField{
			key: strings.Split(typ.Tag.Get("json"), ",")[0], value: value,
			secret: typ.Tag.Get("provider") == "secret",
		})
	}
	return fields
}

func (f notificationField) desired() any {
	switch f.value.Elem().Kind() {
	case reflect.String:
		return f.value.Elem().String()
	case reflect.Bool:
		return f.value.Elem().Bool()
	case reflect.Int:
		return int(f.value.Elem().Int())
	default:
		panic("unsupported notification input type")
	}
}

func (f notificationField) read(settings coolify.NotificationSettings) (any, bool, error) {
	value, present := settings[f.key]
	if !present {
		return nil, false, nil
	}
	if f.secret {
		if text, ok := value.(string); ok && text != "" && strings.Trim(text, "*") == "" {
			return nil, false, nil
		}
	}
	switch f.value.Type().Elem().Kind() {
	case reflect.String:
		if value == nil {
			return "", true, nil
		}
		if text, ok := value.(string); ok {
			return text, true, nil
		}
	case reflect.Bool:
		if flag, ok := value.(bool); ok {
			return flag, true, nil
		}
		// Several Coolify model fields lack boolean casts. PDO may return
		// those tinyints as JSON numbers or strings, depending on the driver.
		if value == float64(0) || value == "0" {
			return false, true, nil
		}
		if value == float64(1) || value == "1" {
			return true, true, nil
		}
	case reflect.Int:
		if value == nil {
			return nil, true, nil
		}
		if number, ok := value.(float64); ok && number == math.Trunc(number) && math.Abs(number) <= float64(1<<53-1) {
			return int(number), true, nil
		}
	}
	// Do not include the response value: it might contain a secret.
	return nil, false, fmt.Errorf("coolify notification field %s has an unexpected type", f.key)
}

func applyNotification[A any](ctx context.Context, channel string, current coolify.NotificationSettings, previous, desired A) error {
	previousValues := map[string]any{}
	for _, field := range notificationFields(&previous, channel, false) {
		if !field.value.IsNil() {
			previousValues[field.key] = field.desired()
		}
	}
	patch := coolify.NotificationSettings{}
	for _, field := range notificationFields(&desired, channel, false) {
		if field.value.IsNil() {
			continue
		}
		want := field.desired()
		have, readable, err := field.read(current)
		if err != nil {
			return err
		}
		if field.secret && !readable {
			have, readable = previousValues[field.key]
		}
		if readable && reflect.DeepEqual(want, have) {
			continue
		}
		if want == "" {
			// Coolify's nullable URL/email fields reject the empty string.
			patch[field.key] = nil
		} else {
			patch[field.key] = want
		}
	}
	if len(patch) == 0 {
		return nil
	}
	updated, err := client(ctx).UpdateNotificationSettings(ctx, channel, patch)
	if err != nil {
		return err
	}
	_, err = notificationIdentity(updated, channel, strconv.Itoa(int(current["team_id"].(float64)))+"/"+channel)
	return err
}
