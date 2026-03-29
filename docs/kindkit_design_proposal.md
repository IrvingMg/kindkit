# Design Proposal: kindkit — A Go Library for Managing Kind Clusters

## Summary

This document proposes **kindkit**, a Go library for managing the lifecycle of **Kind** clusters inside Go-based end-to-end test workflows.

The main audience is teams that run operator or other Go-based end-to-end tests against real Kind clusters and currently create those clusters with shell scripts, Makefiles, or similar tools. A second audience is Go projects that already use Kind’s Go API and keep writing the same lifecycle helper code.

kindkit provides an in-process Go API for the most common cluster lifecycle tasks, such as creating, reusing, deleting, and inspecting Kind clusters from Go code. The first version focuses on the most common needs in test workflows. It does not try to solve every Kind workflow from the start, but it should be able to grow over time as new common use cases become clear.

kindkit is not a shared API for multiple cluster providers, and it is not a general testing framework.

## Problem

Many operator and e2e test suites manage Kind clusters through shell scripts, usually with `hack/e2e-test.sh`, Makefiles, or similar wrappers. This creates a split workflow:

- shell scripts manage cluster lifecycle,
- Go code manages tests and controller interaction,
- failures must cross a process and language boundary,
- cleanup and debugging become harder to keep consistent.

This causes several practical problems:

- lifecycle failures are harder to report and debug than normal Go errors,
- test setup and teardown are harder to connect cleanly to Go test code,
- partial failures can leave clusters behind or lose useful debug data,
- projects often rewrite similar helper code for kubeconfig handling, readiness checks, image loading, and cleanup.

Kind already provides a Go package at `sigs.k8s.io/kind/pkg/cluster`. It gives the main building blocks for cluster creation and deletion, but it does not define shared lifecycle behavior for test-focused workflows, such as reuse, readiness checks, cleanup, and common debug flows.

As a result, projects that want in-process lifecycle management often build their own wrappers for:

- cluster creation and readiness checks,
- cluster reuse decisions,
- kubeconfig conversion to `*rest.Config`,
- cleanup and teardown helpers,
- image loading,
- test integration.

The main problem is not only duplicate boilerplate. It is duplicate failure handling, cleanup logic, and repeated rules for creating, reusing, and cleaning up clusters across many projects.

## Goals / Non-Goals

### Goals

For the first version, kindkit should:

1. Provide a focused Go API for common Kind lifecycle operations, including **create, reuse, delete, and inspect**.
2. Fit naturally into Go-based test workflows.
3. Expose kubeconfig data in forms that are useful for Go callers.
4. Support loading host container images into cluster nodes.
5. Make failure handling and log collection clear and explicit.
6. Start with a focused public API that can expand carefully over time.

### Non-Goals

For the first version, kindkit will not:

1. Abstract over multiple cluster providers.
2. Become a general-purpose testing framework.
3. Replace Kind or extend Kind upstream.
4. Hide all Kind concepts behind a generic API.
5. Cover every Kind workflow or configuration option from the start.

## Proposed Design

The proposal centers on one main concept: a **Cluster** managed through a small lifecycle API.

The library is responsible for a focused set of capabilities:

- create a cluster,
- optionally reuse an existing cluster,
- delete a cluster,
- expose kubeconfig information in forms useful to Go callers,
- load images into nodes,
- export logs for debugging.

This proposal defines the intended first-version shape. It is focused on the most common lifecycle operations needed by Go end-to-end workflows that run against real Kind clusters. It does not define a final limit for the library. Additional helpers and lifecycle operations can be added later if repeated real workflows show that they belong in the core package.

The following API sketch shows that first-version shape:

```go
type Cluster struct {
    name     string
    provider *cluster.Provider
}

func Create(ctx context.Context, name string, opts ...Option) (*Cluster, error)
func CreateOrReuse(ctx context.Context, name string, opts ...Option) (*Cluster, error)

func (c *Cluster) Name() string
func (c *Cluster) Delete(ctx context.Context) error

func (c *Cluster) KubeconfigPath() (string, error)
func (c *Cluster) RESTConfig() (*rest.Config, error)

func (c *Cluster) LoadImages(ctx context.Context, images ...string) error
func (c *Cluster) ExportLogs(ctx context.Context, dir string) error
```

For common cases, kindkit may also provide a small helper type that describes the cluster layout the caller wants.

For example:

```go
type ClusterConfig struct {
    Workers int
}

kindkit.Create(ctx, "test-cluster",
    kindkit.WithClusterConfig(kindkit.ClusterConfig{Workers: 2}),
)
```

This helper is only meant for common layouts in the first version. It should stay small and practical. If repeated use cases show that more fields belong there, the type can grow over time.

Callers that need more control should also be able to pass upstream Kind configuration directly through an escape hatch in the option layer.

### Example workflow

```go
func TestMain(m *testing.M) {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
    defer cancel()

    c, err := kindkit.Create(ctx, "my-operator-test",
        kindkit.WithNodeImage("kindest/node:v1.31.0@sha256:..."),
        kindkit.WithWaitForReady(5*time.Minute),
    )
    if err != nil {
        if c != nil {
            _ = c.ExportLogs(context.Background(), "./test-logs")
            _ = c.Delete(context.Background())
        }
        log.Fatal(err)
    }

    if err := c.LoadImages(ctx, "my-operator:latest"); err != nil {
        _ = c.Delete(context.Background())
        log.Fatal(err)
    }

    restConfig, err := c.RESTConfig()
    if err != nil {
        _ = c.Delete(context.Background())
        log.Fatal(err)
    }

    _ = restConfig

    code := m.Run()

    if err := c.Delete(context.Background()); err != nil {
        log.Printf("failed to delete kind cluster: %v", err)
    }

    os.Exit(code)
}
```

## Key Decisions

### The first API is focused, but not closed

kindkit should start with the most common lifecycle operations for Kind-based test workflows. This keeps the first version easier to understand and maintain.

At the same time, this proposal does not assume that the first API is the final API. New surface area can be added later when repeated real-world workflows show that it belongs in the core library.

### Reuse is based on name and basic health in the first version

`CreateOrReuse` reuses an existing cluster when:

- a cluster with the requested name exists, and
- the cluster meets the library’s initial reuse readiness criteria.

The first definition is intentionally simple: the API server is reachable. This gives a practical starting point for the first version.

If real workflows show that this is too weak, the reuse policy can become stricter later, for example by checking more cluster state or selected configuration inputs.

kindkit does **not** compare the full desired configuration with the existing cluster before deciding to reuse it in the first version. If callers need strict config matching, they must delete and recreate the cluster themselves.

### Failures should still leave enough state for debugging

Cluster creation can fail at different points:

- before any usable cluster exists,
- after the cluster has been created but before it is ready,
- after creation succeeds but later setup fails.

If a cluster has already been created but is not fully ready, the API may still return the `*Cluster` together with an error. This lets callers collect logs, inspect the cluster, and clean it up instead of losing useful debug state.

### Cleanup must be explicit and test-friendly

kindkit should support common teardown patterns in Go test workflows, but it should not hide cleanup failures. Deletion failures should stay visible in test output.

### Kubeconfig access should support common use cases

Different callers need kubeconfig in different forms. For the first version, the most useful forms are a filesystem path and a `*rest.Config`. These support common test and client setup flows without adding unnecessary API surface.

If a strong need for raw kubeconfig bytes appears later, that can be added without changing the core lifecycle model.

### Image loading is a core requirement

Image loading is a core requirement for operator and integration test workflows. kindkit should support it through standard Kind-based operations without making it a large part of the public API.

### Kind is the only target

kindkit is intentionally focused on Kind. The immediate problem is improving Kind-based test workflows in Go. Adding support for multiple providers would make the API and maintenance work larger without helping the main use case of this proposal.

## Design Principles for Growth

The library should stay focused, but it should not stay artificially small.

New API surface should be added only when repeated real-world workflows show that it belongs in kindkit’s core scope.

That core scope is Kind cluster lifecycle management from Go test code. The library may grow within that scope over time, but it should not become a provider-neutral abstraction or a general end-to-end test framework.

## Tradeoffs

This design accepts several downsides.

### Another abstraction layer

kindkit adds another library between callers and Kind. This improves lifecycle consistency and ease of use, but users must learn kindkit behavior instead of using only Kind primitives.

### The first version does not cover every workflow

A focused first version keeps the design smaller and easier to adopt, but some users may still need direct Kind usage for advanced or uncommon workflows.

### No provider-neutral abstraction

kindkit stays focused on Kind instead of defining a shared API for many cluster providers. This keeps the design simpler and closer to the main use case, but it also means the library is less useful for projects that want one abstraction across different local cluster backends.

### Protecting users from Kind changes adds maintenance work

If kindkit tries to reduce the effect of upstream Kind changes on its users, it must carry some of that maintenance work itself.

## Risks

### Upstream API and implementation changes

Kind’s Go API has been fairly stable in practice, but it is not presented as a strong long-term external API promise for libraries like this. kindkit should assume that some nearby behavior may change over time.

### Dependency coupling

Importing Kind also brings Kubernetes- and runtime-related dependencies. Even if kindkit adds only a small layer on top, consumers may still face version pressure or dependency conflicts.

## Alternatives

### Continue using shell scripts

This is still the lowest-effort option, but it keeps the same split workflow and weaker lifecycle visibility that this proposal is trying to improve.

### Use Kind’s Go API directly in each project

This avoids adding a new dependency, but each project still needs to define lifecycle behavior, cleanup rules, image-loading strategy, and test integration on its own.

### Use a larger framework

Frameworks such as e2e-framework can help in this area, but they solve a different problem. e2e-framework is a test framework with its own test structure, while kindkit is a Kind lifecycle library that can be used with standard `go test`, Ginkgo, or other Go-based workflows.

### Build a shared API for multiple providers

This sounds more general, but it weakens the proposal. The immediate need is specifically about Kind-based workflows, and multi-provider support would increase API and maintenance complexity too early.

## Testing

The implementation should include both unit and integration coverage for the main lifecycle flows.

Unit tests should cover option handling, config translation, and lifecycle error paths.

Integration tests should cover cluster creation, reuse, deletion, image loading, log export, and partial-failure behavior.

## Conclusion

kindkit should provide a focused first-version Go API for Kind cluster lifecycle management in test workflows.

The value of the proposal is not that it keeps the library permanently small. The value is that it starts with the most common lifecycle operations, keeps failure handling and cleanup explicit, and leaves room to grow later when repeated use cases show that more functionality belongs in the core package.
