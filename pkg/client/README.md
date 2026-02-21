# Registry Client

Package `client` provides an SDK for microservices to register with the Alvas API Gateway.

## Features

- Service registration with route definitions
- Automatic heartbeat to maintain registration
- Graceful shutdown with deregistration
- Retry with exponential backoff
- Route collision detection and handling

## Installation

```go
import "github.com/alvas/api-api/pkg/client"
```

## Usage

### Basic Registration

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "github.com/apascualco/gotway/pkg/client"
)

func main() {
    // Create the registry client
    registryClient := client.NewRegistryClient(
        "http://api-gateway:8080",
        os.Getenv("REGISTRY_SERVICE_TOKEN"),
    )

    // Register the service
    ctx := context.Background()
    resp, err := registryClient.Register(ctx, client.RegisterRequest{
        ServiceName: "user-service",
        Host:        "user-service",
        Port:        8081,
        BasePath:    "/api/v1/users",
        Version:     "1.0.0",
        HealthURL:   "/health",
        Routes: []client.Route{
            {Method: "GET", Path: "", Public: false, Scopes: []string{"users:read"}},
            {Method: "POST", Path: "", Public: false, Scopes: []string{"users:write"}},
            {Method: "GET", Path: "/:id", Public: false, Scopes: []string{"users:read"}},
            {Method: "PUT", Path: "/:id", Public: false, Scopes: []string{"users:write"}},
            {Method: "DELETE", Path: "/:id", Public: false, Scopes: []string{"users:delete"}},
        },
        Metadata: map[string]string{
            "team": "platform",
        },
    })
    if err != nil {
        log.Fatalf("Failed to register: %v", err)
    }

    log.Printf("Registered with instance ID: %s", resp.InstanceID)

    // Wait for shutdown signal
    quit := make(chan os.Signal, 1)
    signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
    <-quit

    // Graceful shutdown
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    if err := registryClient.Shutdown(shutdownCtx); err != nil {
        log.Printf("Shutdown error: %v", err)
    }
}
```

### Handling Route Collisions

```go
resp, err := registryClient.Register(ctx, req)
if err != nil {
    if collisionErr, ok := err.(*client.CollisionError); ok {
        for _, collision := range collisionErr.Collisions {
            log.Printf("Route collision: %s %s already registered by %s",
                collision.Method, collision.Path, collision.RegisteredBy)
        }
        return
    }
    log.Fatalf("Registration failed: %v", err)
}
```

### Custom Configuration

```go
registryClient := client.NewRegistryClient(
    gatewayURL,
    token,
    client.WithTimeout(30*time.Second),
    client.WithLogger(customLogger),
    client.WithHTTPClient(customHTTPClient),
)
```

## API

### Types

```go
type Route struct {
    Method    string   // HTTP method (GET, POST, PUT, DELETE, etc.)
    Path      string   // Route path relative to BasePath
    Public    bool     // If true, no authentication required
    RateLimit int      // Rate limit for this route (0 = use default)
    Scopes    []string // Required scopes for authentication
}

type RegisterRequest struct {
    ServiceName string            // Unique service identifier
    Host        string            // Service hostname
    Port        int               // Service port
    HealthURL   string            // Health check endpoint (default: /health)
    Version     string            // Service version
    BasePath    string            // Base path for all routes
    Routes      []Route           // Routes to register
    Metadata    map[string]string // Optional metadata
}

type RegisterResponse struct {
    InstanceID        string   // Assigned instance ID
    HeartbeatInterval int      // Heartbeat interval in seconds
    HeartbeatURL      string   // Heartbeat endpoint
    RegisteredRoutes  []string // List of registered routes
}
```

### Methods

- `NewRegistryClient(gatewayURL, token string, opts ...Option) *RegistryClient` - Create a new client
- `Register(ctx context.Context, req RegisterRequest) (*RegisterResponse, error)` - Register the service
- `Deregister(ctx context.Context) error` - Deregister the service
- `Shutdown(ctx context.Context) error` - Graceful shutdown (stops heartbeat + deregisters)
- `InstanceID() string` - Get the current instance ID

## Environment Variables

```bash
# Gateway URL
GATEWAY_URL=http://api-gateway:8080

# Service token for authentication
REGISTRY_SERVICE_TOKEN=your-secret-token
```

## Error Handling

The client handles several error types:

- `*CollisionError` - Route conflicts with existing registrations (not retried)
- Network errors - Automatically retried with exponential backoff
- Authentication errors (401) - Retried (may be transient)
- Server errors (5xx) - Retried with exponential backoff

## Retry Behavior

The client uses exponential backoff for retries:

- Initial backoff: 1 second
- Maximum backoff: 30 seconds
- Maximum retries: 5
- Backoff multiplier: 2x

Collision errors are never retried since they require manual resolution.
