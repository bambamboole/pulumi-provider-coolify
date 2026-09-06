package provider

import (
	"context"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/infer"
)

// NotificationEvents selects which events a channel delivers. Unset fields
// remain managed by Coolify; false explicitly disables an event.
type NotificationEvents struct {
	DeploymentSuccess    *bool `pulumi:"deploymentSuccess,optional" json:"deployment_success,omitempty"`
	DeploymentFailure    *bool `pulumi:"deploymentFailure,optional" json:"deployment_failure,omitempty"`
	StatusChange         *bool `pulumi:"statusChange,optional" json:"status_change,omitempty"`
	RestartLimitReached  *bool `pulumi:"restartLimitReached,optional" json:"restart_limit_reached,omitempty"`
	BackupSuccess        *bool `pulumi:"backupSuccess,optional" json:"backup_success,omitempty"`
	BackupFailure        *bool `pulumi:"backupFailure,optional" json:"backup_failure,omitempty"`
	ScheduledTaskSuccess *bool `pulumi:"scheduledTaskSuccess,optional" json:"scheduled_task_success,omitempty"`
	ScheduledTaskFailure *bool `pulumi:"scheduledTaskFailure,optional" json:"scheduled_task_failure,omitempty"`
	DockerCleanupSuccess *bool `pulumi:"dockerCleanupSuccess,optional" json:"docker_cleanup_success,omitempty"`
	DockerCleanupFailure *bool `pulumi:"dockerCleanupFailure,optional" json:"docker_cleanup_failure,omitempty"`
	ServerDiskUsage      *bool `pulumi:"serverDiskUsage,optional" json:"server_disk_usage,omitempty"`
	ServerReachable      *bool `pulumi:"serverReachable,optional" json:"server_reachable,omitempty"`
	ServerUnreachable    *bool `pulumi:"serverUnreachable,optional" json:"server_unreachable,omitempty"`
	ServerPatch          *bool `pulumi:"serverPatch,optional" json:"server_patch,omitempty"`
	TraefikOutdated      *bool `pulumi:"traefikOutdated,optional" json:"traefik_outdated,omitempty"`
}

func (args *NotificationEvents) Annotate(a infer.Annotator) {
	a.Describe(&args, "Notification event selection. Omitted events keep their Coolify settings; false disables an event.")
	a.Describe(&args.DeploymentSuccess, "Deployment Success notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.DeploymentFailure, "Deployment Failure notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.StatusChange, "Status Change notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.RestartLimitReached, "Restart Limit Reached notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.BackupSuccess, "Backup Success notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.BackupFailure, "Backup Failure notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.ScheduledTaskSuccess, "Scheduled Task Success notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.ScheduledTaskFailure, "Scheduled Task Failure notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.DockerCleanupSuccess, "Docker Cleanup Success notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.DockerCleanupFailure, "Docker Cleanup Failure notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.ServerDiskUsage, "Server Disk Usage notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.ServerReachable, "Server Reachable notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.ServerUnreachable, "Server Unreachable notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.ServerPatch, "Server Patch notifications. Leave unset to preserve the current setting.")
	a.Describe(&args.TraefikOutdated, "Traefik Outdated notifications. Leave unset to preserve the current setting.")
}

// NotificationTelegramThreads routes each event to a Telegram topic/thread.
type NotificationTelegramThreads struct {
	DeploymentSuccess    *string `pulumi:"deploymentSuccess,optional" json:"deployment_success,omitempty" provider:"secret"`
	DeploymentFailure    *string `pulumi:"deploymentFailure,optional" json:"deployment_failure,omitempty" provider:"secret"`
	StatusChange         *string `pulumi:"statusChange,optional" json:"status_change,omitempty" provider:"secret"`
	RestartLimitReached  *string `pulumi:"restartLimitReached,optional" json:"restart_limit_reached,omitempty" provider:"secret"`
	BackupSuccess        *string `pulumi:"backupSuccess,optional" json:"backup_success,omitempty" provider:"secret"`
	BackupFailure        *string `pulumi:"backupFailure,optional" json:"backup_failure,omitempty" provider:"secret"`
	ScheduledTaskSuccess *string `pulumi:"scheduledTaskSuccess,optional" json:"scheduled_task_success,omitempty" provider:"secret"`
	ScheduledTaskFailure *string `pulumi:"scheduledTaskFailure,optional" json:"scheduled_task_failure,omitempty" provider:"secret"`
	DockerCleanupSuccess *string `pulumi:"dockerCleanupSuccess,optional" json:"docker_cleanup_success,omitempty" provider:"secret"`
	DockerCleanupFailure *string `pulumi:"dockerCleanupFailure,optional" json:"docker_cleanup_failure,omitempty" provider:"secret"`
	ServerDiskUsage      *string `pulumi:"serverDiskUsage,optional" json:"server_disk_usage,omitempty" provider:"secret"`
	ServerReachable      *string `pulumi:"serverReachable,optional" json:"server_reachable,omitempty" provider:"secret"`
	ServerUnreachable    *string `pulumi:"serverUnreachable,optional" json:"server_unreachable,omitempty" provider:"secret"`
	ServerPatch          *string `pulumi:"serverPatch,optional" json:"server_patch,omitempty" provider:"secret"`
	TraefikOutdated      *string `pulumi:"traefikOutdated,optional" json:"traefik_outdated,omitempty" provider:"secret"`
}

func (args *NotificationTelegramThreads) Annotate(a infer.Annotator) {
	a.Describe(&args, "Telegram thread IDs by event. Unset fields remain unmanaged; an empty string clears the thread ID.")
	a.Describe(&args.DeploymentSuccess, "Thread ID for deployment success notifications.")
	a.Describe(&args.DeploymentFailure, "Thread ID for deployment failure notifications.")
	a.Describe(&args.StatusChange, "Thread ID for status change notifications.")
	a.Describe(&args.RestartLimitReached, "Thread ID for restart limit reached notifications.")
	a.Describe(&args.BackupSuccess, "Thread ID for backup success notifications.")
	a.Describe(&args.BackupFailure, "Thread ID for backup failure notifications.")
	a.Describe(&args.ScheduledTaskSuccess, "Thread ID for scheduled task success notifications.")
	a.Describe(&args.ScheduledTaskFailure, "Thread ID for scheduled task failure notifications.")
	a.Describe(&args.DockerCleanupSuccess, "Thread ID for docker cleanup success notifications.")
	a.Describe(&args.DockerCleanupFailure, "Thread ID for docker cleanup failure notifications.")
	a.Describe(&args.ServerDiskUsage, "Thread ID for server disk usage notifications.")
	a.Describe(&args.ServerReachable, "Thread ID for server reachable notifications.")
	a.Describe(&args.ServerUnreachable, "Thread ID for server unreachable notifications.")
	a.Describe(&args.ServerPatch, "Thread ID for server patch notifications.")
	a.Describe(&args.TraefikOutdated, "Thread ID for traefik outdated notifications.")
}

// NotificationSMTPEncryption is the transport encryption accepted by Coolify.
type NotificationSMTPEncryption string

const (
	NotificationSMTPEncryptionSTARTTLS NotificationSMTPEncryption = "starttls"
	NotificationSMTPEncryptionTLS      NotificationSMTPEncryption = "tls"
	NotificationSMTPEncryptionNone     NotificationSMTPEncryption = "none"
)

func (NotificationSMTPEncryption) Values() []infer.EnumValue[NotificationSMTPEncryption] {
	return []infer.EnumValue[NotificationSMTPEncryption]{
		{Name: "STARTTLS", Value: NotificationSMTPEncryptionSTARTTLS},
		{Name: "TLS", Value: NotificationSMTPEncryptionTLS},
		{Name: "None", Value: NotificationSMTPEncryptionNone},
	}
}

// NotificationSlack manages the slack settings singleton for the API token's team.
type NotificationSlack struct{}

type NotificationSlackArgs struct {
	Enabled    *bool               `pulumi:"enabled,optional" json:"slack_enabled,omitempty"`
	WebhookURL *string             `pulumi:"webhookUrl,optional" json:"slack_webhook_url,omitempty" provider:"secret"`
	Events     *NotificationEvents `pulumi:"events,optional" json:"events,omitempty"`
}

type NotificationSlackState struct {
	NotificationSlackArgs
	TeamID *int `pulumi:"teamId"`
}

func (r *NotificationSlack) Annotate(a infer.Annotator) {
	a.SetToken("index", "NotificationSlack")
	a.Describe(&r, "Slack-compatible notifications, including Mattermost incoming webhooks. Manages one settings object per API token team on Coolify v4.3.0+. Create adopts existing settings. Only declared fields are changed. Delete disables delivery and retains credentials and event choices.")
}

func (args *NotificationSlackArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Enabled, "slack enabled. Omit to leave the current setting unmanaged.")
	a.Describe(&args.WebhookURL, "slack webhook url. Treated as a Pulumi secret. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.Events, "Events delivered through this channel. Only explicitly set event flags are managed.")
}

func (state *NotificationSlackState) Annotate(a infer.Annotator) {
	a.Describe(&state.TeamID, "The Coolify team ID derived from the provider's API token. Import using <teamId>/slack.")
}

func notificationSlackState(args NotificationSlackArgs, teamID int) NotificationSlackState {
	return NotificationSlackState{NotificationSlackArgs: args, TeamID: &teamID}
}

func (NotificationSlack) Create(ctx context.Context, req infer.CreateRequest[NotificationSlackArgs]) (infer.CreateResponse[NotificationSlackState], error) {
	return createNotification(ctx, "slack", req, notificationSlackState)
}

func (NotificationSlack) Diff(_ context.Context, req infer.DiffRequest[NotificationSlackArgs, NotificationSlackState]) (infer.DiffResponse, error) {
	return diffResponse(diffArgs(req.State.NotificationSlackArgs, req.Inputs), true), nil
}

func (NotificationSlack) Update(ctx context.Context, req infer.UpdateRequest[NotificationSlackArgs, NotificationSlackState]) (infer.UpdateResponse[NotificationSlackState], error) {
	return updateNotification(ctx, "slack", req, req.State.NotificationSlackArgs, notificationTeamID(req.State.TeamID), notificationSlackState)
}

func (NotificationSlack) Read(ctx context.Context, req infer.ReadRequest[NotificationSlackArgs, NotificationSlackState]) (infer.ReadResponse[NotificationSlackArgs, NotificationSlackState], error) {
	return readNotification(ctx, "slack", req, req.State.TeamID == nil, notificationSlackState)
}

func (NotificationSlack) Delete(ctx context.Context, req infer.DeleteRequest[NotificationSlackState]) (infer.DeleteResponse, error) {
	return deleteNotification(ctx, "slack", req.ID)
}

// NotificationDiscord manages the discord settings singleton for the API token's team.
type NotificationDiscord struct{}

type NotificationDiscordArgs struct {
	Enabled     *bool               `pulumi:"enabled,optional" json:"discord_enabled,omitempty"`
	WebhookURL  *string             `pulumi:"webhookUrl,optional" json:"discord_webhook_url,omitempty" provider:"secret"`
	PingEnabled *bool               `pulumi:"pingEnabled,optional" json:"discord_ping_enabled,omitempty"`
	Events      *NotificationEvents `pulumi:"events,optional" json:"events,omitempty"`
}

type NotificationDiscordState struct {
	NotificationDiscordArgs
	TeamID *int `pulumi:"teamId"`
}

func (r *NotificationDiscord) Annotate(a infer.Annotator) {
	a.SetToken("index", "NotificationDiscord")
	a.Describe(&r, "Discord webhook notifications and optional pings. Manages one settings object per API token team on Coolify v4.3.0+. Create adopts existing settings. Only declared fields are changed. Delete disables delivery and retains credentials and event choices.")
}

func (args *NotificationDiscordArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Enabled, "discord enabled. Omit to leave the current setting unmanaged.")
	a.Describe(&args.WebhookURL, "discord webhook url. Treated as a Pulumi secret. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.PingEnabled, "discord ping enabled. Omit to leave the current setting unmanaged.")
	a.Describe(&args.Events, "Events delivered through this channel. Only explicitly set event flags are managed.")
}

func (state *NotificationDiscordState) Annotate(a infer.Annotator) {
	a.Describe(&state.TeamID, "The Coolify team ID derived from the provider's API token. Import using <teamId>/discord.")
}

func notificationDiscordState(args NotificationDiscordArgs, teamID int) NotificationDiscordState {
	return NotificationDiscordState{NotificationDiscordArgs: args, TeamID: &teamID}
}

func (NotificationDiscord) Create(ctx context.Context, req infer.CreateRequest[NotificationDiscordArgs]) (infer.CreateResponse[NotificationDiscordState], error) {
	return createNotification(ctx, "discord", req, notificationDiscordState)
}

func (NotificationDiscord) Diff(_ context.Context, req infer.DiffRequest[NotificationDiscordArgs, NotificationDiscordState]) (infer.DiffResponse, error) {
	return diffResponse(diffArgs(req.State.NotificationDiscordArgs, req.Inputs), true), nil
}

func (NotificationDiscord) Update(ctx context.Context, req infer.UpdateRequest[NotificationDiscordArgs, NotificationDiscordState]) (infer.UpdateResponse[NotificationDiscordState], error) {
	return updateNotification(ctx, "discord", req, req.State.NotificationDiscordArgs, notificationTeamID(req.State.TeamID), notificationDiscordState)
}

func (NotificationDiscord) Read(ctx context.Context, req infer.ReadRequest[NotificationDiscordArgs, NotificationDiscordState]) (infer.ReadResponse[NotificationDiscordArgs, NotificationDiscordState], error) {
	return readNotification(ctx, "discord", req, req.State.TeamID == nil, notificationDiscordState)
}

func (NotificationDiscord) Delete(ctx context.Context, req infer.DeleteRequest[NotificationDiscordState]) (infer.DeleteResponse, error) {
	return deleteNotification(ctx, "discord", req.ID)
}

// NotificationEmail manages the email settings singleton for the API token's team.
type NotificationEmail struct{}

type NotificationEmailArgs struct {
	SMTPEnabled              *bool                       `pulumi:"smtpEnabled,optional" json:"smtp_enabled,omitempty"`
	SMTPFromAddress          *string                     `pulumi:"smtpFromAddress,optional" json:"smtp_from_address,omitempty" provider:"secret"`
	SMTPFromName             *string                     `pulumi:"smtpFromName,optional" json:"smtp_from_name,omitempty" provider:"secret"`
	SMTPRecipients           *string                     `pulumi:"smtpRecipients,optional" json:"smtp_recipients,omitempty" provider:"secret"`
	SMTPHost                 *string                     `pulumi:"smtpHost,optional" json:"smtp_host,omitempty" provider:"secret"`
	SMTPPort                 *int                        `pulumi:"smtpPort,optional" json:"smtp_port,omitempty"`
	SMTPEncryption           *NotificationSMTPEncryption `pulumi:"smtpEncryption,optional" json:"smtp_encryption,omitempty"`
	SMTPUsername             *string                     `pulumi:"smtpUsername,optional" json:"smtp_username,omitempty" provider:"secret"`
	SMTPPassword             *string                     `pulumi:"smtpPassword,optional" json:"smtp_password,omitempty" provider:"secret"`
	SMTPTimeout              *int                        `pulumi:"smtpTimeout,optional" json:"smtp_timeout,omitempty"`
	SMTPEHLODomain           *string                     `pulumi:"smtpEhloDomain,optional" json:"smtp_ehlo_domain,omitempty"`
	ResendEnabled            *bool                       `pulumi:"resendEnabled,optional" json:"resend_enabled,omitempty"`
	ResendAPIKey             *string                     `pulumi:"resendApiKey,optional" json:"resend_api_key,omitempty" provider:"secret"`
	UseInstanceEmailSettings *bool                       `pulumi:"useInstanceEmailSettings,optional" json:"use_instance_email_settings,omitempty"`
	Events                   *NotificationEvents         `pulumi:"events,optional" json:"events,omitempty"`
}

type NotificationEmailState struct {
	NotificationEmailArgs
	TeamID *int `pulumi:"teamId"`
}

func (r *NotificationEmail) Annotate(a infer.Annotator) {
	a.SetToken("index", "NotificationEmail")
	a.Describe(&r, "Team email notifications through SMTP, Resend or instance email settings. Manages one settings object per API token team on Coolify v4.3.0+. Create adopts existing settings. Only declared fields are changed. Delete disables delivery and retains credentials and event choices.")
}

func (args *NotificationEmailArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.SMTPEnabled, "smtp enabled. Omit to leave the current setting unmanaged.")
	a.Describe(&args.SMTPFromAddress, "smtp from address. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.SMTPFromName, "smtp from name. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.SMTPRecipients, "smtp recipients. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.SMTPHost, "smtp host. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.SMTPPort, "smtp port. Omit to leave the current setting unmanaged.")
	a.Describe(&args.SMTPEncryption, "SMTP transport encryption: starttls, tls or none. Omit to leave the current setting unmanaged.")
	a.Describe(&args.SMTPUsername, "smtp username. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.SMTPPassword, "smtp password. Treated as a Pulumi secret. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.SMTPTimeout, "smtp timeout. Omit to leave the current setting unmanaged.")
	a.Describe(&args.SMTPEHLODomain, "smtp ehlo domain. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.ResendEnabled, "resend enabled. Omit to leave the current setting unmanaged.")
	a.Describe(&args.ResendAPIKey, "resend api key. Treated as a Pulumi secret. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.UseInstanceEmailSettings, "use instance email settings. Omit to leave the current setting unmanaged.")
	a.Describe(&args.Events, "Events delivered through this channel. Only explicitly set event flags are managed.")
}

func (state *NotificationEmailState) Annotate(a infer.Annotator) {
	a.Describe(&state.TeamID, "The Coolify team ID derived from the provider's API token. Import using <teamId>/email.")
}

func notificationEmailState(args NotificationEmailArgs, teamID int) NotificationEmailState {
	return NotificationEmailState{NotificationEmailArgs: args, TeamID: &teamID}
}

func (NotificationEmail) Check(ctx context.Context, req infer.CheckRequest) (infer.CheckResponse[NotificationEmailArgs], error) {
	args, failures, err := infer.DefaultCheck[NotificationEmailArgs](ctx, req.NewInputs)
	if err != nil {
		return infer.CheckResponse[NotificationEmailArgs]{}, err
	}
	if args.SMTPEncryption != nil && !req.NewInputs.Get("smtpEncryption").IsComputed() {
		switch *args.SMTPEncryption {
		case "", NotificationSMTPEncryptionSTARTTLS, NotificationSMTPEncryptionTLS, NotificationSMTPEncryptionNone:
		default:
			failures = append(failures, p.CheckFailure{Property: "smtpEncryption", Reason: "must be starttls, tls or none"})
		}
	}
	if args.SMTPPort != nil && !req.NewInputs.Get("smtpPort").IsComputed() && (*args.SMTPPort < 1 || *args.SMTPPort > 65535) {
		failures = append(failures, p.CheckFailure{Property: "smtpPort", Reason: "must be between 1 and 65535"})
	}
	if args.SMTPTimeout != nil && !req.NewInputs.Get("smtpTimeout").IsComputed() && *args.SMTPTimeout < 0 {
		failures = append(failures, p.CheckFailure{Property: "smtpTimeout", Reason: "must be non-negative"})
	}
	return infer.CheckResponse[NotificationEmailArgs]{Inputs: args, Failures: failures}, nil
}

func (NotificationEmail) Create(ctx context.Context, req infer.CreateRequest[NotificationEmailArgs]) (infer.CreateResponse[NotificationEmailState], error) {
	return createNotification(ctx, "email", req, notificationEmailState)
}

func (NotificationEmail) Diff(_ context.Context, req infer.DiffRequest[NotificationEmailArgs, NotificationEmailState]) (infer.DiffResponse, error) {
	return diffResponse(diffArgs(req.State.NotificationEmailArgs, req.Inputs), true), nil
}

func (NotificationEmail) Update(ctx context.Context, req infer.UpdateRequest[NotificationEmailArgs, NotificationEmailState]) (infer.UpdateResponse[NotificationEmailState], error) {
	return updateNotification(ctx, "email", req, req.State.NotificationEmailArgs, notificationTeamID(req.State.TeamID), notificationEmailState)
}

func (NotificationEmail) Read(ctx context.Context, req infer.ReadRequest[NotificationEmailArgs, NotificationEmailState]) (infer.ReadResponse[NotificationEmailArgs, NotificationEmailState], error) {
	return readNotification(ctx, "email", req, req.State.TeamID == nil, notificationEmailState)
}

func (NotificationEmail) Delete(ctx context.Context, req infer.DeleteRequest[NotificationEmailState]) (infer.DeleteResponse, error) {
	return deleteNotification(ctx, "email", req.ID)
}

// NotificationTelegram manages the telegram settings singleton for the API token's team.
type NotificationTelegram struct{}

type NotificationTelegramArgs struct {
	Enabled *bool                        `pulumi:"enabled,optional" json:"telegram_enabled,omitempty"`
	Token   *string                      `pulumi:"token,optional" json:"telegram_token,omitempty" provider:"secret"`
	ChatID  *string                      `pulumi:"chatId,optional" json:"telegram_chat_id,omitempty" provider:"secret"`
	Threads *NotificationTelegramThreads `pulumi:"threads,optional" json:"threads,omitempty"`
	Events  *NotificationEvents          `pulumi:"events,optional" json:"events,omitempty"`
}

type NotificationTelegramState struct {
	NotificationTelegramArgs
	TeamID *int `pulumi:"teamId"`
}

func (r *NotificationTelegram) Annotate(a infer.Annotator) {
	a.SetToken("index", "NotificationTelegram")
	a.Describe(&r, "Telegram bot notifications with optional per-event topic/thread IDs. Manages one settings object per API token team on Coolify v4.3.0+. Create adopts existing settings. Only declared fields are changed. Delete disables delivery and retains credentials and event choices.")
}

func (args *NotificationTelegramArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Enabled, "telegram enabled. Omit to leave the current setting unmanaged.")
	a.Describe(&args.Token, "telegram token. Treated as a Pulumi secret. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.ChatID, "telegram chat id. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.Threads, "Per-event Telegram thread IDs. Omit to leave the current setting unmanaged.")
	a.Describe(&args.Events, "Events delivered through this channel. Only explicitly set event flags are managed.")
}

func (state *NotificationTelegramState) Annotate(a infer.Annotator) {
	a.Describe(&state.TeamID, "The Coolify team ID derived from the provider's API token. Import using <teamId>/telegram.")
}

func notificationTelegramState(args NotificationTelegramArgs, teamID int) NotificationTelegramState {
	return NotificationTelegramState{NotificationTelegramArgs: args, TeamID: &teamID}
}

func (NotificationTelegram) Create(ctx context.Context, req infer.CreateRequest[NotificationTelegramArgs]) (infer.CreateResponse[NotificationTelegramState], error) {
	return createNotification(ctx, "telegram", req, notificationTelegramState)
}

func (NotificationTelegram) Diff(_ context.Context, req infer.DiffRequest[NotificationTelegramArgs, NotificationTelegramState]) (infer.DiffResponse, error) {
	return diffResponse(diffArgs(req.State.NotificationTelegramArgs, req.Inputs), true), nil
}

func (NotificationTelegram) Update(ctx context.Context, req infer.UpdateRequest[NotificationTelegramArgs, NotificationTelegramState]) (infer.UpdateResponse[NotificationTelegramState], error) {
	return updateNotification(ctx, "telegram", req, req.State.NotificationTelegramArgs, notificationTeamID(req.State.TeamID), notificationTelegramState)
}

func (NotificationTelegram) Read(ctx context.Context, req infer.ReadRequest[NotificationTelegramArgs, NotificationTelegramState]) (infer.ReadResponse[NotificationTelegramArgs, NotificationTelegramState], error) {
	return readNotification(ctx, "telegram", req, req.State.TeamID == nil, notificationTelegramState)
}

func (NotificationTelegram) Delete(ctx context.Context, req infer.DeleteRequest[NotificationTelegramState]) (infer.DeleteResponse, error) {
	return deleteNotification(ctx, "telegram", req.ID)
}

// NotificationPushover manages the pushover settings singleton for the API token's team.
type NotificationPushover struct{}

type NotificationPushoverArgs struct {
	Enabled  *bool               `pulumi:"enabled,optional" json:"pushover_enabled,omitempty"`
	UserKey  *string             `pulumi:"userKey,optional" json:"pushover_user_key,omitempty" provider:"secret"`
	APIToken *string             `pulumi:"apiToken,optional" json:"pushover_api_token,omitempty" provider:"secret"`
	Events   *NotificationEvents `pulumi:"events,optional" json:"events,omitempty"`
}

type NotificationPushoverState struct {
	NotificationPushoverArgs
	TeamID *int `pulumi:"teamId"`
}

func (r *NotificationPushover) Annotate(a infer.Annotator) {
	a.SetToken("index", "NotificationPushover")
	a.Describe(&r, "Pushover push notifications. Manages one settings object per API token team on Coolify v4.3.0+. Create adopts existing settings. Only declared fields are changed. Delete disables delivery and retains credentials and event choices.")
}

func (args *NotificationPushoverArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Enabled, "pushover enabled. Omit to leave the current setting unmanaged.")
	a.Describe(&args.UserKey, "pushover user key. Treated as a Pulumi secret. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.APIToken, "pushover api token. Treated as a Pulumi secret. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.Events, "Events delivered through this channel. Only explicitly set event flags are managed.")
}

func (state *NotificationPushoverState) Annotate(a infer.Annotator) {
	a.Describe(&state.TeamID, "The Coolify team ID derived from the provider's API token. Import using <teamId>/pushover.")
}

func notificationPushoverState(args NotificationPushoverArgs, teamID int) NotificationPushoverState {
	return NotificationPushoverState{NotificationPushoverArgs: args, TeamID: &teamID}
}

func (NotificationPushover) Create(ctx context.Context, req infer.CreateRequest[NotificationPushoverArgs]) (infer.CreateResponse[NotificationPushoverState], error) {
	return createNotification(ctx, "pushover", req, notificationPushoverState)
}

func (NotificationPushover) Diff(_ context.Context, req infer.DiffRequest[NotificationPushoverArgs, NotificationPushoverState]) (infer.DiffResponse, error) {
	return diffResponse(diffArgs(req.State.NotificationPushoverArgs, req.Inputs), true), nil
}

func (NotificationPushover) Update(ctx context.Context, req infer.UpdateRequest[NotificationPushoverArgs, NotificationPushoverState]) (infer.UpdateResponse[NotificationPushoverState], error) {
	return updateNotification(ctx, "pushover", req, req.State.NotificationPushoverArgs, notificationTeamID(req.State.TeamID), notificationPushoverState)
}

func (NotificationPushover) Read(ctx context.Context, req infer.ReadRequest[NotificationPushoverArgs, NotificationPushoverState]) (infer.ReadResponse[NotificationPushoverArgs, NotificationPushoverState], error) {
	return readNotification(ctx, "pushover", req, req.State.TeamID == nil, notificationPushoverState)
}

func (NotificationPushover) Delete(ctx context.Context, req infer.DeleteRequest[NotificationPushoverState]) (infer.DeleteResponse, error) {
	return deleteNotification(ctx, "pushover", req.ID)
}

// NotificationWebhook manages the webhook settings singleton for the API token's team.
type NotificationWebhook struct{}

type NotificationWebhookArgs struct {
	Enabled    *bool               `pulumi:"enabled,optional" json:"webhook_enabled,omitempty"`
	WebhookURL *string             `pulumi:"webhookUrl,optional" json:"webhook_url,omitempty" provider:"secret"`
	Events     *NotificationEvents `pulumi:"events,optional" json:"events,omitempty"`
}

type NotificationWebhookState struct {
	NotificationWebhookArgs
	TeamID *int `pulumi:"teamId"`
}

func (r *NotificationWebhook) Annotate(a infer.Annotator) {
	a.SetToken("index", "NotificationWebhook")
	a.Describe(&r, "Generic JSON webhook notifications. Manages one settings object per API token team on Coolify v4.3.0+. Create adopts existing settings. Only declared fields are changed. Delete disables delivery and retains credentials and event choices.")
}

func (args *NotificationWebhookArgs) Annotate(a infer.Annotator) {
	a.Describe(&args.Enabled, "webhook enabled. Omit to leave the current setting unmanaged.")
	a.Describe(&args.WebhookURL, "webhook url. Treated as a Pulumi secret. Omit to leave the current setting unmanaged. An empty string clears the setting.")
	a.Describe(&args.Events, "Events delivered through this channel. Only explicitly set event flags are managed.")
}

func (state *NotificationWebhookState) Annotate(a infer.Annotator) {
	a.Describe(&state.TeamID, "The Coolify team ID derived from the provider's API token. Import using <teamId>/webhook.")
}

func notificationWebhookState(args NotificationWebhookArgs, teamID int) NotificationWebhookState {
	return NotificationWebhookState{NotificationWebhookArgs: args, TeamID: &teamID}
}

func (NotificationWebhook) Create(ctx context.Context, req infer.CreateRequest[NotificationWebhookArgs]) (infer.CreateResponse[NotificationWebhookState], error) {
	return createNotification(ctx, "webhook", req, notificationWebhookState)
}

func (NotificationWebhook) Diff(_ context.Context, req infer.DiffRequest[NotificationWebhookArgs, NotificationWebhookState]) (infer.DiffResponse, error) {
	return diffResponse(diffArgs(req.State.NotificationWebhookArgs, req.Inputs), true), nil
}

func (NotificationWebhook) Update(ctx context.Context, req infer.UpdateRequest[NotificationWebhookArgs, NotificationWebhookState]) (infer.UpdateResponse[NotificationWebhookState], error) {
	return updateNotification(ctx, "webhook", req, req.State.NotificationWebhookArgs, notificationTeamID(req.State.TeamID), notificationWebhookState)
}

func (NotificationWebhook) Read(ctx context.Context, req infer.ReadRequest[NotificationWebhookArgs, NotificationWebhookState]) (infer.ReadResponse[NotificationWebhookArgs, NotificationWebhookState], error) {
	return readNotification(ctx, "webhook", req, req.State.TeamID == nil, notificationWebhookState)
}

func (NotificationWebhook) Delete(ctx context.Context, req infer.DeleteRequest[NotificationWebhookState]) (infer.DeleteResponse, error) {
	return deleteNotification(ctx, "webhook", req.ID)
}
