package provider

import (
	"testing"

	p "github.com/pulumi/pulumi-go-provider"
	"github.com/pulumi/pulumi/sdk/v3/go/property"
)

func TestNotificationRootTeamEmptyInputsAndImport(t *testing.T) {
	for _, tt := range notificationChannels {
		t.Run(tt.channel, func(t *testing.T) {
			fake := newNotificationAPIFake(t, tt.channel, map[string]any{tt.enabled: true})
			fake.settings["team_id"] = 0
			server := notificationServer(t, fake)
			urn := notificationURN(tt.token)
			inputs := notificationCheck(t, server, urn, map[string]property.Value{})
			created, err := server.Create(p.CreateRequest{Urn: urn, Properties: inputs})
			if err != nil {
				t.Fatal(err)
			}
			refreshed, err := server.Read(p.ReadRequest{Urn: urn, ID: created.ID, Properties: created.Properties, Inputs: inputs})
			if err != nil {
				t.Fatal(err)
			}
			enabled := "enabled"
			if tt.channel == "email" {
				enabled = "smtpEnabled"
			}
			if !refreshed.Inputs.Get(enabled).IsNull() {
				t.Fatal("refresh adopted an undeclared setting for root team")
			}
			imported, err := server.Read(p.ReadRequest{Urn: urn, ID: "0/" + tt.channel})
			if err != nil {
				t.Fatal(err)
			}
			if flag := imported.Inputs.Get(enabled); !flag.IsBool() || !flag.AsBool() {
				t.Fatal("import did not discover root team settings")
			}
			notificationAssertPatches(t, fake)
		})
	}
}

func TestNotificationRejectsInvalidTeamBeforeWriting(t *testing.T) {
	for _, team := range []any{nil, float64(-1), 1.5, "invalid", float64(1 << 53)} {
		fake := newNotificationAPIFake(t, "slack", map[string]any{"slack_enabled": false})
		fake.settings["team_id"] = team
		server := notificationServer(t, fake)
		urn := notificationURN("NotificationSlack")
		inputs := notificationCheck(t, server, urn, map[string]property.Value{"enabled": property.New(true)})
		if _, err := server.Create(p.CreateRequest{Urn: urn, Properties: inputs}); err == nil {
			t.Fatalf("accepted invalid team ID %v", team)
		}
		notificationAssertPatches(t, fake)
	}
}
