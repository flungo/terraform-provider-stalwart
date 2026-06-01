// Copyright (c) Fabrizio Lungo
// SPDX-License-Identifier: MPL-2.0

// Package acctest provides a self-contained test harness that boots a real
// Stalwart server in a container and exposes its management API to acceptance
// tests, so that `make testacc` requires no externally-provisioned instance.
//
// The harness drives the `docker` CLI directly through os/exec (rather than a
// Docker SDK) so it has no extra module dependencies and works against any
// daemon the local `docker` binary can reach. Stalwart is started in
// **recovery mode**, which serves the full JMAP management API over plain HTTP
// with no TLS, no setup wizard, and no mail services — the minimal surface the
// provider talks to. See CLAUDE.md for the background.
package acctest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultImage is the Stalwart image used when STALWART_TEST_IMAGE is unset.
	// Pinned to a minor series so the harness tracks a single API version; the
	// env var allows testing against other versions (e.g. a future v0.17).
	DefaultImage = "stalwartlabs/stalwart:v0.16"

	// imageEnv overrides the image under test.
	imageEnv = "STALWART_TEST_IMAGE"

	// adminUser and adminPassword are the pinned recovery-administrator
	// credentials. STALWART_RECOVERY_ADMIN bypasses the directory, so these are
	// valid for the management API without any further provisioning.
	adminUser     = "admin"
	adminPassword = "testpassword"

	// managementPort is the in-container HTTP management/recovery port.
	managementPort = "8080"
)

// Container represents a running Stalwart test container.
type Container struct {
	id       string
	endpoint string // base URL, e.g. http://127.0.0.1:49160
	image    string
}

// Image returns the image the container was started from.
func (c *Container) Image() string { return c.image }

// Endpoint returns the base URL of the Stalwart management API (without the
// trailing JMAP path, which the provider appends).
func (c *Container) Endpoint() string { return c.endpoint }

// AdminUser returns the recovery-administrator username.
func (c *Container) AdminUser() string { return adminUser }

// AdminPassword returns the recovery-administrator password.
func (c *Container) AdminPassword() string { return adminPassword }

// image resolves the image to run, honouring the STALWART_TEST_IMAGE override.
func image() string {
	if v := os.Getenv(imageEnv); v != "" {
		return v
	}
	return DefaultImage
}

// Start launches a Stalwart container in recovery mode and waits for its
// management API to become ready. The caller must call Terminate when done.
func Start(ctx context.Context) (*Container, error) {
	if err := ensureDocker(ctx); err != nil {
		return nil, err
	}

	img := image()
	if err := ensureImage(ctx, img); err != nil {
		return nil, err
	}

	// Run detached, publishing the management port to an ephemeral host port so
	// concurrent runs don't collide. A minimal config.json (a single RocksDB
	// DataStore object) is injected so the server starts in recovery mode rather
	// than the interactive bootstrap wizard.
	//
	// The config file is written via an entrypoint override: we cannot easily
	// bind-mount a host file (the test host path may not be visible to a remote
	// daemon), so the container command writes it before exec'ing the server.
	//
	// The container is deliberately NOT started with --rm: if the server exits
	// early (bad binary path, rejected config, etc.) we need it to stick around
	// so `docker logs` can explain why. Teardown removes it explicitly.
	runArgs := []string{
		"run", "-d",
		"-P", // publish exposed ports to random host ports
		"-e", "STALWART_RECOVERY_MODE=1",
		"-e", "STALWART_RECOVERY_ADMIN=" + adminUser + ":" + adminPassword,
		"-e", "STALWART_RECOVERY_MODE_PORT=" + managementPort,
		"--entrypoint", "/bin/sh",
		img,
		"-c", bootstrapScript(),
	}
	out, err := dockerOutput(ctx, runArgs...)
	if err != nil {
		return nil, fmt.Errorf("starting container: %w", err)
	}
	id := strings.TrimSpace(out)
	c := &Container{id: id, image: img}

	// Guard every failure path from here on by removing the container and
	// attaching its logs, so a CI failure is diagnosable from the test output.
	// The logs are also written to STALWART_TEST_LOG_DIR (when set) so CI can
	// upload them as an artifact, since the raw step output is not always
	// retrievable.
	fail := func(err error) (*Container, error) {
		logs, _ := dockerOutput(context.Background(), "logs", id)
		writeDiagnostics(id, err, logs)
		_ = terminate(context.Background(), id)
		return nil, fmt.Errorf("%w\n--- container logs ---\n%s", err, logs)
	}

	// Confirm the container did not exit immediately (the common failure when the
	// startup command is wrong); a fast exit would otherwise surface only as an
	// empty published port.
	if err := ensureRunning(ctx, id); err != nil {
		return fail(err)
	}

	hostPort, err := publishedPort(ctx, id, managementPort)
	if err != nil {
		return fail(err)
	}
	c.endpoint = "http://127.0.0.1:" + hostPort

	if err := waitReady(ctx, c); err != nil {
		return fail(fmt.Errorf("waiting for Stalwart to become ready: %w", err))
	}

	return c, nil
}

// DumpLogs writes the container's current logs to STALWART_TEST_LOG_DIR (if
// set), so that test failures caused by server-side rejections during CRUD are
// diagnosable after the container is torn down. It is a best-effort aid; errors
// are ignored.
func (c *Container) DumpLogs(ctx context.Context) {
	if c == nil || c.id == "" {
		return
	}
	dir := os.Getenv(logDirEnv)
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	logs, _ := dockerOutput(ctx, "logs", c.id)
	_ = os.WriteFile(filepath.Join(dir, "stalwart-server.log"), []byte(logs), 0o644)
}

// logDirEnv names a directory into which failure diagnostics are written, so CI
// can upload them as an artifact when the raw step log is not retrievable.
const logDirEnv = "STALWART_TEST_LOG_DIR"

// writeDiagnostics persists the failure reason and container logs to a file in
// STALWART_TEST_LOG_DIR, if that variable is set. Errors are ignored: this is a
// best-effort diagnostic aid.
func writeDiagnostics(id string, cause error, logs string) {
	dir := os.Getenv(logDirEnv)
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}
	content := fmt.Sprintf("container: %s\nerror: %v\n\n--- container logs ---\n%s\n", id, cause, logs)
	_ = os.WriteFile(filepath.Join(dir, "stalwart-startup.log"), []byte(content), 0o644)
}

// ensureRunning verifies the container is still running shortly after launch,
// catching startup commands that exit immediately. It returns an error
// describing the exit (with state) if the container is not running.
func ensureRunning(ctx context.Context, id string) error {
	// Give the process a brief moment to fail fast if it is going to.
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(2 * time.Second):
	}
	out, err := dockerOutput(ctx, "inspect", "-f", "{{.State.Running}} {{.State.ExitCode}}", id)
	if err != nil {
		return fmt.Errorf("inspecting container state: %w", err)
	}
	state := strings.TrimSpace(out)
	if !strings.HasPrefix(state, "true") {
		return fmt.Errorf("container exited during startup (running exitCode = %q)", state)
	}
	return nil
}

// Terminate stops and removes the container. It is safe to call more than once.
func (c *Container) Terminate(ctx context.Context) error {
	if c == nil || c.id == "" {
		return nil
	}
	err := terminate(ctx, c.id)
	c.id = ""
	return err
}

// bootstrapScript writes the minimal config.json and execs the server. Stalwart
// reads config.json (the DataStore location) and, with STALWART_RECOVERY_MODE
// set, serves only the management API.
//
// Paths live under /tmp because the image runs as an unprivileged user (UID
// 2000) that cannot necessarily write elsewhere. The server binary is resolved
// with `command -v stalwart` rather than a hard-coded path so the harness is
// resilient to the image laying the binary out differently across versions.
func bootstrapScript() string {
	return strings.Join([]string{
		`set -e`,
		`mkdir -p /tmp/stalwart-data`,
		`printf '%s' '{"@type":"RocksDb","path":"/tmp/stalwart-data/"}' > /tmp/stalwart-config.json`,
		`bin="$(command -v stalwart || echo /usr/local/bin/stalwart)"`,
		`exec "$bin" --config /tmp/stalwart-config.json`,
	}, "; ")
}

// terminate force-removes a container by id (stopping it first if needed).
func terminate(ctx context.Context, id string) error {
	if _, err := dockerOutput(ctx, "rm", "-f", id); err != nil {
		return fmt.Errorf("removing container %s: %w", id, err)
	}
	return nil
}

// dockerOutput runs `docker <args...>` and returns combined stdout, surfacing
// stderr in the error on failure.
func dockerOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return stdout.String(), fmt.Errorf("docker %s: %w: %s",
			strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

// ensureDocker verifies a reachable Docker daemon. It does not attempt to start
// one: daemon management is environment-specific (see CLAUDE.md) and is handled
// by the Makefile target / CI, not the test process.
func ensureDocker(ctx context.Context) error {
	if _, err := exec.LookPath("docker"); err != nil {
		return fmt.Errorf("docker binary not found on PATH: %w", err)
	}
	if _, err := dockerOutput(ctx, "info", "--format", "{{.ServerVersion}}"); err != nil {
		return fmt.Errorf("docker daemon not reachable: %w", err)
	}
	return nil
}

// ensureImage pulls the image if it is not already present locally.
func ensureImage(ctx context.Context, img string) error {
	if _, err := dockerOutput(ctx, "image", "inspect", img); err == nil {
		return nil // already present
	}
	pullCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	if _, err := dockerOutput(pullCtx, "pull", img); err != nil {
		return fmt.Errorf("pulling %s: %w", img, err)
	}
	return nil
}

// publishedPort returns the host port that the given container port is mapped
// to.
func publishedPort(ctx context.Context, id, containerPort string) (string, error) {
	format := fmt.Sprintf(`{{(index (index .NetworkSettings.Ports "%s/tcp") 0).HostPort}}`, containerPort)
	out, err := dockerOutput(ctx, "inspect", "-f", format, id)
	if err != nil {
		return "", fmt.Errorf("inspecting published port: %w", err)
	}
	port := strings.TrimSpace(out)
	if port == "" {
		return "", fmt.Errorf("container %s did not publish port %s", id, containerPort)
	}
	return port, nil
}
