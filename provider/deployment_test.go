package provider

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestRunDeploymentWaitsForTerminalStatus(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	projectUUID := fake.addProject("Main", "production")
	appUUID := fake.addApplication(map[string]any{"name": "app", "environment_id": fake.environmentID(projectUUID, "production"), "settings": map[string]any{}})

	restore := setDeploymentTiming(time.Millisecond, time.Second, 50*time.Millisecond)
	defer restore()

	go func() {
		time.Sleep(20 * time.Millisecond)
		fake.mu.Lock()
		for _, deployment := range fake.deployments {
			deployment["status"] = "finished"
		}
		fake.mu.Unlock()
	}()
	state, err := runDeployment(context.Background(), c, DeploymentArgs{Application: appUUID, Force: true})
	if err != nil {
		t.Fatalf("runDeployment: %v", err)
	}
	if state.Status != "finished" || state.Commit != "abc123" || !strings.HasPrefix(state.UUID, "dep-") {
		t.Fatalf("unexpected state: %+v", state)
	}
}

func TestWaitForDeploymentFailsOnFailedStatusAndMissingQueueItem(t *testing.T) {
	fake := newFakeCoolify(t)
	c := fake.client()
	restore := setDeploymentTiming(time.Millisecond, time.Second, 20*time.Millisecond)
	defer restore()

	fake.mu.Lock()
	fake.deployments["dep-failed"] = map[string]any{"deployment_uuid": "dep-failed", "status": "failed"}
	fake.mu.Unlock()
	if _, _, err := waitForDeployment(context.Background(), c, "dep-failed"); err == nil || !strings.Contains(err.Error(), `status "failed"`) {
		t.Fatalf("expected failed status error, got %v", err)
	}

	if _, _, err := waitForDeployment(context.Background(), c, "dep-missing"); err == nil || !strings.Contains(err.Error(), "was not found within") {
		t.Fatalf("expected not found grace error, got %v", err)
	}
}

func setDeploymentTiming(poll, timeout, notFoundGrace time.Duration) func() {
	prevPoll, prevTimeout, prevGrace := deploymentPollInterval, deploymentTimeout, deploymentNotFoundGrace
	deploymentPollInterval, deploymentTimeout, deploymentNotFoundGrace = poll, timeout, notFoundGrace
	return func() {
		deploymentPollInterval, deploymentTimeout, deploymentNotFoundGrace = prevPoll, prevTimeout, prevGrace
	}
}
