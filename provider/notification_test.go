package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi-go-provider/integration"
	"github.com/pulumi/pulumi/pkg/v3/codegen/schema"
	"github.com/pulumi/pulumi/sdk/v3/go/common/resource"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

// This fake deliberately serves only the notification singleton API. Any
// unexpected endpoint or HTTP method fails the test, including DELETE.
type notificationAPIFake struct {
	mu       sync.Mutex
	server   *httptest.Server
	settings map[string]any
	patches  []map[string]any
	requests int
	status   int
	hidden   map[string]any
}

func newNotificationAPIFake(t *testing.T, channel string, settings map[string]any) *notificationAPIFake {
	t.Helper()
	f := &notificationAPIFake{settings: settings, hidden: map[string]any{}}
	f.settings["team_id"] = 42
	f.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.requests++
		if r.URL.Path != "/api/v1/notifications/"+channel || (r.Method != http.MethodGet && r.Method != http.MethodPatch) {
			t.Errorf("unhandled API request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected request", http.StatusBadRequest)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Error("API request did not carry provider token")
		}
		w.Header().Set("Content-Type", "application/json")
		if f.status != 0 {
			w.WriteHeader(f.status)
			_, _ = w.Write([]byte(`{"message":"Endpoint unsupported"}`))
			return
		}
		if r.Method == http.MethodPatch {
			var patch map[string]any
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				t.Errorf("decode PATCH: %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			f.patches = append(f.patches, patch)
			for key, value := range patch {
				f.settings[key] = value
			}
		}
		response := make(map[string]any, len(f.settings))
		for key, value := range f.settings {
			response[key] = value
		}
		for key, value := range f.hidden {
			if value == notificationSecretAbsent {
				delete(response, key)
			} else {
				response[key] = value
			}
		}
		if err := json.NewEncoder(w).Encode(response); err != nil {
			t.Errorf("encode GET: %v", err)
		}
	}))
	t.Cleanup(f.server.Close)
	return f
}

const notificationSecretAbsent = "<omitted>"

func notificationServer(t *testing.T, fake *notificationAPIFake) integration.Server {
	t.Helper()
	server := previewProvider(t)
	if err := server.Configure(p.ConfigureRequest{Args: property.NewMap(map[string]property.Value{
		"baseUrl": property.New(fake.server.URL), "apiToken": property.New("test-token"),
	})}); err != nil {
		t.Fatal(err)
	}
	return server
}

func notificationURN(token string) resource.URN {
	return resource.URN("urn:pulumi:test::notifications::coolify:index:" + token + "::settings")
}

func notificationCheck(t *testing.T, server integration.Server, urn resource.URN, values map[string]property.Value) property.Map {
	t.Helper()
	checked, err := server.Check(p.CheckRequest{Urn: urn, Inputs: property.NewMap(values)})
	if err != nil || len(checked.Failures) != 0 {
		t.Fatalf("Check: %v %v", checked.Failures, err)
	}
	return checked.Inputs
}

func notificationAssertPatches(t *testing.T, fake *notificationAPIFake, expected ...map[string]any) {
	t.Helper()
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if !reflect.DeepEqual(fake.patches, expected) {
		t.Fatalf("PATCH bodies = %#v, want %#v", fake.patches, expected)
	}
}

var notificationChannels = []struct{ token, channel, enabled string }{
	{"NotificationSlack", "slack", "slack_enabled"},
	{"NotificationDiscord", "discord", "discord_enabled"},
	{"NotificationEmail", "email", "smtp_enabled"},
	{"NotificationTelegram", "telegram", "telegram_enabled"},
	{"NotificationPushover", "pushover", "pushover_enabled"},
	{"NotificationWebhook", "webhook", "webhook_enabled"},
}

func TestNotificationSingletonLifecycle(t *testing.T) {
	for _, teamID := range []int{0, 42} {
		for _, tt := range notificationChannels {
			t.Run(fmt.Sprintf("%s/team-%d", tt.channel, teamID), func(t *testing.T) {
				failureKey := "deployment_failure_" + tt.channel + "_notifications"
				successKey := "deployment_success_" + tt.channel + "_notifications"
				fake := newNotificationAPIFake(t, tt.channel, map[string]any{tt.enabled: true, failureKey: true, successKey: true})
				fake.settings["team_id"] = teamID
				server := notificationServer(t, fake)
				urn := notificationURN(tt.token)
				enabledInput := "enabled"
				if tt.channel == "email" {
					enabledInput = "smtpEnabled"
				}
				inputs := notificationCheck(t, server, urn, map[string]property.Value{
					enabledInput: property.New(false),
					"events":     property.New(property.NewMap(map[string]property.Value{"deploymentFailure": property.New(false)})),
				})
				created, err := server.Create(p.CreateRequest{Urn: urn, Properties: inputs})
				if err != nil {
					t.Fatal(err)
				}
				if created.ID != fmt.Sprintf("%d/%s", teamID, tt.channel) {
					t.Fatalf("singleton ID = %q", created.ID)
				}
				if v := created.Properties.Get("teamId"); !v.IsNumber() || v.AsNumber() != float64(teamID) {
					t.Fatalf("teamId = %v", v)
				}
				notificationAssertPatches(t, fake, map[string]any{tt.enabled: false, failureKey: false})
				diff, err := server.Diff(p.DiffRequest{Urn: urn, ID: created.ID, State: created.Properties, OldInputs: inputs, Inputs: inputs})
				if err != nil || diff.HasChanges {
					t.Fatalf("unchanged inputs have diff: %+v %v", diff, err)
				}
				updated, err := server.Update(p.UpdateRequest{Urn: urn, ID: created.ID, State: created.Properties, OldInputs: inputs, Inputs: inputs})
				if err != nil {
					t.Fatal(err)
				}
				notificationAssertPatches(t, fake, map[string]any{tt.enabled: false, failureKey: false})
				// A UI change to a declared flag is drift; the undeclared event remains unmanaged.
				fake.mu.Lock()
				fake.settings[failureKey] = true
				fake.settings[successKey] = false
				fake.mu.Unlock()
				refreshed, err := server.Read(p.ReadRequest{Urn: urn, ID: created.ID, Properties: updated.Properties, Inputs: inputs})
				if err != nil {
					t.Fatal(err)
				}
				events := refreshed.Inputs.Get("events").AsMap()
				if !events.Get("deploymentFailure").AsBool() || !events.Get("deploymentSuccess").IsNull() {
					t.Fatalf("refresh must report only declared events: %v", events)
				}
				// Destroy disables the channel and preserves event configuration.
				fake.mu.Lock()
				fake.settings[tt.enabled] = true
				if tt.channel == "email" {
					fake.settings["resend_enabled"] = true
					fake.settings["use_instance_email_settings"] = true
				}
				fake.mu.Unlock()
				if err := server.Delete(p.DeleteRequest{Urn: urn, ID: created.ID, Properties: refreshed.Properties, OldInputs: refreshed.Inputs}); err != nil {
					t.Fatal(err)
				}
				disable := map[string]any{tt.enabled: false}
				if tt.channel == "email" {
					disable["resend_enabled"] = false
					disable["use_instance_email_settings"] = false
				}
				notificationAssertPatches(t, fake, map[string]any{tt.enabled: false, failureKey: false}, disable)
				fake.mu.Lock()
				defer fake.mu.Unlock()
				if fake.settings[failureKey] != true || fake.settings[successKey] != false {
					t.Fatal("destroy changed event settings")
				}
			})
		}
	}
}

func TestNotificationAdoptsWithoutUnnecessaryPatch(t *testing.T) {
	fake := newNotificationAPIFake(t, "slack", map[string]any{"slack_enabled": false, "slack_webhook_url": "https://hooks.example.test/existing"})
	server := notificationServer(t, fake)
	urn := notificationURN("NotificationSlack")
	inputs := notificationCheck(t, server, urn, map[string]property.Value{"enabled": property.New(false)})
	if _, err := server.Create(p.CreateRequest{Urn: urn, Properties: inputs}); err != nil {
		t.Fatal(err)
	}
	notificationAssertPatches(t, fake)
}

var notificationCredentials = []struct{ token, channel, input, api string }{
	{"NotificationSlack", "slack", "webhookUrl", "slack_webhook_url"},
	{"NotificationDiscord", "discord", "webhookUrl", "discord_webhook_url"},
	{"NotificationWebhook", "webhook", "webhookUrl", "webhook_url"},
	{"NotificationTelegram", "telegram", "token", "telegram_token"},
	{"NotificationTelegram", "telegram", "chatId", "telegram_chat_id"},
	{"NotificationPushover", "pushover", "userKey", "pushover_user_key"},
	{"NotificationPushover", "pushover", "apiToken", "pushover_api_token"},
	{"NotificationEmail", "email", "smtpPassword", "smtp_password"},
	{"NotificationEmail", "email", "smtpFromAddress", "smtp_from_address"},
	{"NotificationEmail", "email", "smtpFromName", "smtp_from_name"},
	{"NotificationEmail", "email", "smtpRecipients", "smtp_recipients"},
	{"NotificationEmail", "email", "smtpHost", "smtp_host"},
	{"NotificationEmail", "email", "smtpUsername", "smtp_username"},
	{"NotificationEmail", "email", "resendApiKey", "resend_api_key"},
}

func TestNotificationHiddenCredentialsArePreservedAndRotated(t *testing.T) {
	for _, tt := range notificationCredentials {
		for _, hidden := range []struct {
			name  string
			value any
		}{{"absent", notificationSecretAbsent}, {"masked", "********"}} {
			t.Run(tt.channel+"/"+tt.input+"/"+hidden.name, func(t *testing.T) {
				fake := newNotificationAPIFake(t, tt.channel, map[string]any{tt.api: "old-secret"})
				fake.hidden[tt.api] = hidden.value
				server := notificationServer(t, fake)
				urn := notificationURN(tt.token)
				inputs := notificationCheck(t, server, urn, map[string]property.Value{tt.input: property.New("desired-secret")})
				if !inputs.Get(tt.input).Secret() {
					t.Fatal("Check did not mark credential secret")
				}
				created, err := server.Create(p.CreateRequest{Urn: urn, Properties: inputs})
				if err != nil {
					t.Fatal(err)
				}
				notificationAssertPatches(t, fake, map[string]any{tt.api: "desired-secret"})
				read, err := server.Read(p.ReadRequest{Urn: urn, ID: created.ID, Properties: created.Properties, Inputs: inputs})
				if err != nil {
					t.Fatal(err)
				}
				for _, values := range []property.Map{read.Inputs, read.Properties} {
					if v := values.Get(tt.input); !v.IsString() || v.AsString() != "desired-secret" || !v.Secret() {
						t.Fatalf("read lost secret: %v", v)
					}
				}
				unchanged, err := server.Update(p.UpdateRequest{Urn: urn, ID: created.ID, State: read.Properties, OldInputs: inputs, Inputs: inputs})
				if err != nil {
					t.Fatal(err)
				}
				notificationAssertPatches(t, fake, map[string]any{tt.api: "desired-secret"})
				rotated := inputs.Set(tt.input, property.New("rotated-secret").WithSecret(true))
				changed, err := server.Update(p.UpdateRequest{Urn: urn, ID: created.ID, State: unchanged.Properties, OldInputs: inputs, Inputs: rotated})
				if err != nil {
					t.Fatal(err)
				}
				notificationAssertPatches(t, fake, map[string]any{tt.api: "desired-secret"}, map[string]any{tt.api: "rotated-secret"})
				if v := changed.Properties.Get(tt.input); !v.Secret() || v.AsString() != "rotated-secret" {
					t.Fatalf("update lost rotated secret: %v", v)
				}
				cleared := rotated.Set(tt.input, property.New("").WithSecret(true))
				if _, err := server.Update(p.UpdateRequest{Urn: urn, ID: created.ID, State: changed.Properties, OldInputs: rotated, Inputs: cleared}); err != nil {
					t.Fatal(err)
				}
				notificationAssertPatches(t, fake, map[string]any{tt.api: "desired-secret"}, map[string]any{tt.api: "rotated-secret"}, map[string]any{tt.api: nil})
			})
		}
	}
}

func TestNotificationImportAndUnsupportedAPI(t *testing.T) {
	for _, tt := range notificationChannels {
		t.Run(tt.channel, func(t *testing.T) {
			fake := newNotificationAPIFake(t, tt.channel, map[string]any{tt.enabled: true, "deployment_failure_" + tt.channel + "_notifications": false})
			for _, credential := range notificationCredentials {
				if credential.channel == tt.channel {
					fake.hidden[credential.api] = "********"
				}
			}
			server := notificationServer(t, fake)
			urn := notificationURN(tt.token)
			read, err := server.Read(p.ReadRequest{Urn: urn, ID: "42/" + tt.channel})
			if err != nil {
				t.Fatal(err)
			}
			if read.ID != "42/"+tt.channel || !read.Inputs.Get("events").AsMap().Get("deploymentFailure").IsBool() {
				t.Fatalf("import did not recover API settings: %+v", read)
			}
			for _, credential := range notificationCredentials {
				if credential.channel == tt.channel && (!read.Inputs.Get(credential.input).IsNull() || !read.Properties.Get(credential.input).IsNull()) {
					t.Fatalf("import fabricated unreadable %s", credential.input)
				}
			}
			for _, id := range []string{"43/" + tt.channel, "42/wrong-channel", "42", "invalid/" + tt.channel} {
				if _, err := server.Read(p.ReadRequest{Urn: urn, ID: id, Inputs: read.Inputs, Properties: read.Properties}); err == nil {
					t.Errorf("invalid ID %q accepted", id)
				}
			}
			if err := server.Delete(p.DeleteRequest{Urn: urn, ID: "43/" + tt.channel, Properties: read.Properties}); err == nil {
				t.Error("destroy accepted a different token team")
			}
			if _, err := server.Update(p.UpdateRequest{Urn: urn, ID: "43/" + tt.channel, State: read.Properties, Inputs: read.Inputs}); err == nil {
				t.Error("update accepted a different token team")
			}
			notificationAssertPatches(t, fake)
			fake.mu.Lock()
			fake.status = http.StatusNotFound
			fake.mu.Unlock()
			if _, err := server.Read(p.ReadRequest{Urn: urn, ID: read.ID, Inputs: read.Inputs, Properties: read.Properties}); err == nil {
				t.Error("unsupported notification endpoint was treated as a vanished resource")
			}
		})
	}
}

func TestNotificationPreviewUnknownCredentialsDoesNotCallAPI(t *testing.T) {
	for _, tt := range notificationCredentials {
		t.Run(tt.channel+"/"+tt.input, func(t *testing.T) {
			fake := newNotificationAPIFake(t, tt.channel, map[string]any{})
			server := notificationServer(t, fake)
			urn := notificationURN(tt.token)
			inputs := notificationCheck(t, server, urn, map[string]property.Value{tt.input: property.New(property.Computed).WithSecret(true)})
			if !inputs.Get(tt.input).IsComputed() || !inputs.Get(tt.input).Secret() {
				t.Fatalf("Check lost unknown secret: computed=%v secret=%v value=%v", inputs.Get(tt.input).IsComputed(), inputs.Get(tt.input).Secret(), inputs.Get(tt.input))
			}
			preview, err := server.Create(p.CreateRequest{Urn: urn, Properties: inputs, DryRun: true})
			if err != nil {
				t.Fatal(err)
			}
			if !preview.Properties.Get(tt.input).IsComputed() || !preview.Properties.Get(tt.input).Secret() {
				t.Fatal("create preview lost unknown secret")
			}
			old := property.NewMap(map[string]property.Value{tt.input: property.New("old-secret").WithSecret(true), "teamId": property.New(float64(42))})
			updated, err := server.Update(p.UpdateRequest{Urn: urn, ID: "42/" + tt.channel, State: old, Inputs: inputs, DryRun: true})
			if err != nil {
				t.Fatal(err)
			}
			if !updated.Properties.Get(tt.input).IsComputed() || !updated.Properties.Get(tt.input).Secret() {
				t.Fatal("update preview lost unknown secret")
			}
			fake.mu.Lock()
			defer fake.mu.Unlock()
			if fake.requests != 0 {
				t.Fatalf("preview sent %d API requests", fake.requests)
			}
		})
	}
}

func TestNotificationSchemaMarksCredentialsSecret(t *testing.T) {
	response, err := previewProvider(t).GetSchema(p.GetSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}
	var spec schema.PackageSpec
	if err := json.Unmarshal([]byte(response.Schema), &spec); err != nil {
		t.Fatal(err)
	}
	for _, tt := range notificationCredentials {
		resource, ok := spec.Resources["coolify:index:"+tt.token]
		if !ok {
			t.Errorf("missing resource %s", tt.token)
			continue
		}
		if !resource.InputProperties[tt.input].Secret || !resource.Properties[tt.input].Secret {
			t.Errorf("%s.%s must be secret in inputs and outputs", tt.token, tt.input)
		}
	}
	telegram := spec.Resources["coolify:index:NotificationTelegram"]
	threadsRef := strings.TrimPrefix(telegram.InputProperties["threads"].Ref, "#/types/")
	threads, ok := spec.Types[threadsRef]
	if !ok || len(threads.Properties) != 15 {
		t.Fatalf("Telegram threads schema missing event fields: %q", threadsRef)
	}
	for name, field := range threads.Properties {
		if !field.Secret {
			t.Errorf("Telegram thread %s must be secret", name)
		}
	}

}

func TestNotificationEmailAndTelegramOptionalFieldMapping(t *testing.T) {
	t.Run("email explicit zero and empty clear", func(t *testing.T) {
		fake := newNotificationAPIFake(t, "email", map[string]any{"smtp_timeout": 30, "smtp_from_name": "Old", "smtp_host": "mail.example.test", "resend_enabled": true})
		server := notificationServer(t, fake)
		urn := notificationURN("NotificationEmail")
		inputs := notificationCheck(t, server, urn, map[string]property.Value{"smtpTimeout": property.New(float64(0)), "smtpFromName": property.New(""), "smtpEncryption": property.New("none"), "resendEnabled": property.New(false)})
		if _, err := server.Create(p.CreateRequest{Urn: urn, Properties: inputs}); err != nil {
			t.Fatal(err)
		}
		notificationAssertPatches(t, fake, map[string]any{"smtp_timeout": float64(0), "smtp_from_name": nil, "smtp_encryption": "none", "resend_enabled": false})
	})
	t.Run("telegram thread clear and unmanaged event", func(t *testing.T) {
		fake := newNotificationAPIFake(t, "telegram", map[string]any{"telegram_notifications_deployment_failure_thread_id": "old-thread", "telegram_notifications_backup_success_thread_id": "keep"})
		server := notificationServer(t, fake)
		urn := notificationURN("NotificationTelegram")
		inputs := notificationCheck(t, server, urn, map[string]property.Value{"chatId": property.New("chat"), "threads": property.New(property.NewMap(map[string]property.Value{"deploymentFailure": property.New("")}))})
		if _, err := server.Create(p.CreateRequest{Urn: urn, Properties: inputs}); err != nil {
			t.Fatal(err)
		}
		notificationAssertPatches(t, fake, map[string]any{"telegram_chat_id": "chat", "telegram_notifications_deployment_failure_thread_id": nil})
	})
	t.Run("invalid encryption rejected", func(t *testing.T) {
		checked, err := previewProvider(t).Check(p.CheckRequest{Urn: notificationURN("NotificationEmail"), Inputs: property.NewMap(map[string]property.Value{"smtpEncryption": property.New("invalid")})})
		if err == nil && len(checked.Failures) == 0 {
			t.Fatal("invalid smtpEncryption accepted")
		}
		if err != nil && !strings.Contains(err.Error(), "smtpEncryption") {
			t.Fatalf("unexpected validation error: %v", err)
		}
	})
}

func TestNotificationRemovingInputReleasesManagement(t *testing.T) {
	fake := newNotificationAPIFake(t, "discord", map[string]any{"discord_enabled": true, "discord_ping_enabled": false, "discord_webhook_url": "existing", "deployment_failure_discord_notifications": true})
	server := notificationServer(t, fake)
	urn := notificationURN("NotificationDiscord")
	inputs := notificationCheck(t, server, urn, map[string]property.Value{"enabled": property.New(true), "pingEnabled": property.New(true), "webhookUrl": property.New("existing"), "events": property.New(property.NewMap(map[string]property.Value{"deploymentFailure": property.New(true)}))})
	created, err := server.Create(p.CreateRequest{Urn: urn, Properties: inputs})
	if err != nil {
		t.Fatal(err)
	}
	notificationAssertPatches(t, fake, map[string]any{"discord_ping_enabled": true})
	released := notificationCheck(t, server, urn, map[string]property.Value{"enabled": property.New(true)})
	updated, err := server.Update(p.UpdateRequest{Urn: urn, ID: created.ID, State: created.Properties, OldInputs: inputs, Inputs: released})
	if err != nil {
		t.Fatal(err)
	}
	notificationAssertPatches(t, fake, map[string]any{"discord_ping_enabled": true})
	fake.mu.Lock()
	fake.settings["discord_ping_enabled"] = false
	fake.settings["discord_webhook_url"] = "changed-in-ui"
	fake.settings["deployment_failure_discord_notifications"] = false
	fake.mu.Unlock()
	refreshed, err := server.Read(p.ReadRequest{Urn: urn, ID: created.ID, Properties: updated.Properties, Inputs: released})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"pingEnabled", "webhookUrl", "events"} {
		if !refreshed.Inputs.Get(field).IsNull() {
			t.Errorf("released %s was reintroduced into inputs", field)
		}
	}
	diff, err := server.Diff(p.DiffRequest{Urn: urn, ID: created.ID, State: refreshed.Properties, OldInputs: refreshed.Inputs, Inputs: released})
	if err != nil || diff.HasChanges {
		t.Fatalf("unmanaged UI changes produced drift: %+v %v", diff, err)
	}
}

func TestNotificationDeleteDisablesInstanceEmailSettings(t *testing.T) {
	fake := newNotificationAPIFake(t, "email", map[string]any{"smtp_enabled": false, "resend_enabled": false, "use_instance_email_settings": true})
	server := notificationServer(t, fake)
	urn := notificationURN("NotificationEmail")
	imported, err := server.Read(p.ReadRequest{Urn: urn, ID: "42/email"})
	if err != nil {
		t.Fatal(err)
	}
	if err := server.Delete(p.DeleteRequest{Urn: urn, ID: imported.ID, Properties: imported.Properties}); err != nil {
		t.Fatal(err)
	}
	notificationAssertPatches(t, fake, map[string]any{"use_instance_email_settings": false})
}

func TestNotificationRefreshDetectsClearedVisibleSecret(t *testing.T) {
	fake := newNotificationAPIFake(t, "slack", map[string]any{"slack_webhook_url": "https://hooks.example.test/desired"})
	server := notificationServer(t, fake)
	urn := notificationURN("NotificationSlack")
	inputs := notificationCheck(t, server, urn, map[string]property.Value{"webhookUrl": property.New("https://hooks.example.test/desired")})
	created, err := server.Create(p.CreateRequest{Urn: urn, Properties: inputs})
	if err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	fake.settings["slack_webhook_url"] = nil
	fake.mu.Unlock()
	refreshed, err := server.Read(p.ReadRequest{Urn: urn, ID: created.ID, Properties: created.Properties, Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	for _, values := range []property.Map{refreshed.Inputs, refreshed.Properties} {
		if v := values.Get("webhookUrl"); !v.IsString() || v.AsString() != "" || !v.Secret() {
			t.Fatalf("visible null must report a cleared secret: %v", v)
		}
	}
	diff, err := server.Diff(p.DiffRequest{Urn: urn, ID: created.ID, State: refreshed.Properties, OldInputs: refreshed.Inputs, Inputs: inputs})
	if err != nil || !diff.HasChanges {
		t.Fatalf("cleared secret must produce drift: %+v %v", diff, err)
	}
	if _, err := server.Update(p.UpdateRequest{Urn: urn, ID: created.ID, State: refreshed.Properties, OldInputs: refreshed.Inputs, Inputs: inputs}); err != nil {
		t.Fatal(err)
	}
	notificationAssertPatches(t, fake, map[string]any{"slack_webhook_url": "https://hooks.example.test/desired"})
}

func TestNotificationEmailNullableNumericImportAndDrift(t *testing.T) {
	fake := newNotificationAPIFake(t, "email", map[string]any{"smtp_port": nil, "smtp_timeout": nil})
	server := notificationServer(t, fake)
	urn := notificationURN("NotificationEmail")
	imported, err := server.Read(p.ReadRequest{Urn: urn, ID: "42/email"})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"smtpPort", "smtpTimeout"} {
		if !imported.Inputs.Get(field).IsNull() {
			t.Errorf("imported null %s must remain unset", field)
		}
	}
	inputs := notificationCheck(t, server, urn, map[string]property.Value{"smtpPort": property.New(float64(587)), "smtpTimeout": property.New(float64(30))})
	created, err := server.Create(p.CreateRequest{Urn: urn, Properties: inputs})
	if err != nil {
		t.Fatal(err)
	}
	fake.mu.Lock()
	fake.settings["smtp_port"] = nil
	fake.settings["smtp_timeout"] = nil
	fake.mu.Unlock()
	refreshed, err := server.Read(p.ReadRequest{Urn: urn, ID: created.ID, Properties: created.Properties, Inputs: inputs})
	if err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"smtpPort", "smtpTimeout"} {
		if !refreshed.Inputs.Get(field).IsNull() {
			t.Errorf("cleared managed %s must become unset", field)
		}
	}
	diff, err := server.Diff(p.DiffRequest{Urn: urn, ID: created.ID, State: refreshed.Properties, OldInputs: refreshed.Inputs, Inputs: inputs})
	if err != nil || !diff.HasChanges {
		t.Fatalf("cleared managed numeric settings must produce drift: %+v %v", diff, err)
	}
}

func TestNotificationReadsUncastAPIBooleanFields(t *testing.T) {
	for _, tt := range []struct {
		name  string
		value any
		want  bool
	}{{"zero", 0, false}, {"one", 1, true}, {"string zero", "0", false}, {"string one", "1", true}} {
		t.Run(tt.name, func(t *testing.T) {
			fake := newNotificationAPIFake(t, "slack", map[string]any{"server_reachable_slack_notifications": tt.value, "docker_cleanup_success_slack_notifications": tt.value})
			server := notificationServer(t, fake)
			imported, err := server.Read(p.ReadRequest{Urn: notificationURN("NotificationSlack"), ID: "42/slack"})
			if err != nil {
				t.Fatal(err)
			}
			events := imported.Inputs.Get("events").AsMap()
			for _, field := range []string{"serverReachable", "dockerCleanupSuccess"} {
				if v := events.Get(field); !v.IsBool() || v.AsBool() != tt.want {
					t.Errorf("%s = %v", field, v)
				}
			}
		})
	}
}

func TestNotificationPreviewUnknownTelegramThreadDoesNotCallAPI(t *testing.T) {
	fake := newNotificationAPIFake(t, "telegram", map[string]any{})
	server := notificationServer(t, fake)
	urn := notificationURN("NotificationTelegram")
	inputs := notificationCheck(t, server, urn, map[string]property.Value{"threads": property.New(property.NewMap(map[string]property.Value{"deploymentFailure": property.New(property.Computed).WithSecret(true)}))})
	checkThread := func(values property.Map) {
		t.Helper()
		v := values.Get("threads").AsMap().Get("deploymentFailure")
		if !v.IsComputed() || !v.Secret() {
			t.Fatalf("lost unknown secret thread: %v", v)
		}
	}
	checkThread(inputs)
	created, err := server.Create(p.CreateRequest{Urn: urn, Properties: inputs, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	checkThread(created.Properties)
	updated, err := server.Update(p.UpdateRequest{Urn: urn, ID: "42/telegram", State: property.NewMap(map[string]property.Value{"teamId": property.New(float64(42))}), Inputs: inputs, DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	checkThread(updated.Properties)
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.requests != 0 {
		t.Fatalf("preview sent %d API requests", fake.requests)
	}
}
