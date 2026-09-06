package coolify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/bambamboole/pulumi-provider-coolify/internal/coolify/api"
)

// NotificationSettings contains the current team's settings for one channel.
// Coolify's specification leaves the response untyped. JSON numbers decode as
// float64, and sensitive fields may be omitted depending on token permissions.
type NotificationSettings map[string]any

// GetNotificationSettings reads a channel's settings for the token's team.
// Coolify initializes the team's settings with defaults if they do not exist.
func (c *Client) GetNotificationSettings(ctx context.Context, channel string) (NotificationSettings, error) {
	switch channel {
	case "email":
		return decode[NotificationSettings](c.api.GetCurrentTeamEmailNotifications(ctx))
	case "discord":
		return decode[NotificationSettings](c.api.GetCurrentTeamDiscordNotifications(ctx))
	case "slack":
		return decode[NotificationSettings](c.api.GetCurrentTeamSlackNotifications(ctx))
	case "telegram":
		return decode[NotificationSettings](c.api.GetCurrentTeamTelegramNotifications(ctx))
	case "pushover":
		return decode[NotificationSettings](c.api.GetCurrentTeamPushoverNotifications(ctx))
	case "webhook":
		return decode[NotificationSettings](c.api.GetCurrentTeamWebhookNotifications(ctx))
	default:
		return nil, fmt.Errorf("coolify: unsupported notification channel %q", channel)
	}
}

// UpdateNotificationSettings applies only fields present in patch and returns
// the updated settings. Explicit false, zero and null values are preserved.
func (c *Client) UpdateNotificationSettings(ctx context.Context, channel string, patch NotificationSettings) (NotificationSettings, error) {
	var update func(context.Context, string, io.Reader, ...api.RequestEditorFn) (*http.Response, error)
	switch channel {
	case "email":
		update = c.api.UpdateCurrentTeamEmailNotificationsWithBody
	case "discord":
		update = c.api.UpdateCurrentTeamDiscordNotificationsWithBody
	case "slack":
		update = c.api.UpdateCurrentTeamSlackNotificationsWithBody
	case "telegram":
		update = c.api.UpdateCurrentTeamTelegramNotificationsWithBody
	case "pushover":
		update = c.api.UpdateCurrentTeamPushoverNotificationsWithBody
	case "webhook":
		update = c.api.UpdateCurrentTeamWebhookNotificationsWithBody
	default:
		return nil, fmt.Errorf("coolify: unsupported notification channel %q", channel)
	}
	if patch == nil {
		patch = NotificationSettings{}
	}
	body, err := json.Marshal(patch)
	if err != nil {
		// Marshal errors can contain values from the request, including secrets.
		return nil, fmt.Errorf("coolify: encode %s notification settings: invalid JSON value", channel)
	}
	return decode[NotificationSettings](update(ctx, "application/json", bytes.NewReader(body)))
}
