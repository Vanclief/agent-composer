package sandboxes

import (
	"context"
	"testing"
	"time"
)

func TestPresence(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	now := time.Now()

	owner := insertTask(t, env.db, "owner")
	idle := insertTask(t, env.db, "idle")
	insertBooted(t, env.db, owner, "up", now)
	insertBooted(t, env.db, owner, "down", now)
	insertBooted(t, env.db, idle, "parked", now)
	env.rt.statuses["up"] = "running"
	env.rt.statuses["down"] = "stopped"
	env.rt.statuses["parked"] = "stopped"

	running, found := env.api.SandboxRunning(ctx, "up")
	if !running || !found {
		t.Fatalf("up: running %v found %v", running, found)
	}

	running, found = env.api.SandboxRunning(ctx, "down")
	if running || !found {
		t.Fatalf("down: running %v found %v", running, found)
	}

	running, found = env.api.SandboxRunning(ctx, "nowhere")
	if running || found {
		t.Fatalf("nowhere: running %v found %v", running, found)
	}

	if !env.api.TaskRunning(ctx, "owner") {
		t.Fatal("owner has a running sandbox")
	}

	if env.api.TaskRunning(ctx, "idle") || env.api.TaskRunning(ctx, "missing") {
		t.Fatal("idle and missing tasks run nothing")
	}
}
