package integration

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/apascualco/gotway/internal/infrastructure/config"
	gatewayhttp "github.com/apascualco/gotway/internal/infrastructure/http"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testServer     *gatewayhttp.Server
	testServerURL  string
	testConfig     *config.Config
	redisContainer testcontainers.Container
	serviceToken   = "test-service-token"
)

func TestMain(m *testing.M) {
	ctx := context.Background()

	if err := setupRedisContainer(ctx); err != nil {
		log.Fatalf("failed to setup redis container: %v", err)
	}

	if err := setupGateway(); err != nil {
		log.Fatalf("failed to setup api: %v", err)
	}

	code := m.Run()

	cleanup(ctx)
	os.Exit(code)
}

func setupRedisContainer(ctx context.Context) error {
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForLog("Ready to accept connections"),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return fmt.Errorf("failed to start redis container: %w", err)
	}
	redisContainer = container

	host, err := container.Host(ctx)
	if err != nil {
		return fmt.Errorf("failed to get redis host: %w", err)
	}

	port, err := container.MappedPort(ctx, "6379")
	if err != nil {
		return fmt.Errorf("failed to get redis port: %w", err)
	}

	os.Setenv("REDIS_URL", fmt.Sprintf("redis://%s:%s/0", host, port.Port()))
	return nil
}

func setupGateway() error {
	port, err := getFreePort()
	if err != nil {
		return fmt.Errorf("failed to get free port: %w", err)
	}

	os.Setenv("PORT", fmt.Sprintf("%d", port))
	os.Setenv("SERVICE_TOKEN", serviceToken)
	os.Setenv("ENV", "test")
	os.Setenv("LOG_LEVEL", "error")
	os.Setenv("RATE_LIMIT_ENABLED", "true")
	os.Setenv("RATE_LIMIT_GLOBAL_RPM", "1000")
	os.Setenv("RATE_LIMIT_USER_RPM", "100")
	os.Setenv("RATE_LIMIT_IP_RPM", "60")

	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
	testConfig = cfg

	server, err := gatewayhttp.NewServer(cfg)
	if err != nil {
		return fmt.Errorf("failed to create server: %w", err)
	}
	testServer = server

	go func() {
		if err := server.Run(); err != nil && err != http.ErrServerClosed {
			log.Printf("server error: %v", err)
		}
	}()

	testServerURL = fmt.Sprintf("http://localhost:%d", port)

	if err := waitForServer(testServerURL, 10*time.Second); err != nil {
		return fmt.Errorf("server didn't start in time: %w", err)
	}

	return nil
}

func cleanup(ctx context.Context) {
	if testServer != nil {
		shutdownCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		testServer.Shutdown(shutdownCtx)
	}

	if redisContainer != nil {
		redisContainer.Terminate(ctx)
	}
}

func getFreePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func waitForServer(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/health")
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("server did not respond within %v", timeout)
}

func getHTTPClient() *http.Client {
	return &http.Client{Timeout: 5 * time.Second}
}
