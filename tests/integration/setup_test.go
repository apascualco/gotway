package integration

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/apascualco/gotway/internal/infrastructure/config"
	gatewayhttp "github.com/apascualco/gotway/internal/infrastructure/http"
	jwtlib "github.com/golang-jwt/jwt/v5"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	testServer     *gatewayhttp.Server
	testServerURL  string
	redisContainer testcontainers.Container
	testPrivateKey *rsa.PrivateKey
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

	if err := os.Setenv("REDIS_URL", fmt.Sprintf("redis://%s:%s/0", host, port.Port())); err != nil {
		return fmt.Errorf("failed to set REDIS_URL: %w", err)
	}
	return nil
}

func setupGateway() error {
	port, err := getFreePort()
	if err != nil {
		return fmt.Errorf("failed to get free port: %w", err)
	}

	// Generate RSA key pair for JWT service-to-service auth
	testPrivateKey, err = rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return fmt.Errorf("failed to generate RSA key: %w", err)
	}

	privBytes := x509.MarshalPKCS1PrivateKey(testPrivateKey)
	privPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: privBytes,
	})

	pubBytes, err := x509.MarshalPKIXPublicKey(&testPrivateKey.PublicKey)
	if err != nil {
		return fmt.Errorf("failed to marshal public key: %w", err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	if err := os.Setenv("PORT", fmt.Sprintf("%d", port)); err != nil {
		return fmt.Errorf("failed to set PORT: %w", err)
	}
	if err := os.Setenv("ENV", "test"); err != nil {
		return fmt.Errorf("failed to set ENV: %w", err)
	}
	if err := os.Setenv("LOG_LEVEL", "error"); err != nil {
		return fmt.Errorf("failed to set LOG_LEVEL: %w", err)
	}
	if err := os.Setenv("RATE_LIMIT_ENABLED", "true"); err != nil {
		return fmt.Errorf("failed to set RATE_LIMIT_ENABLED: %w", err)
	}
	if err := os.Setenv("RATE_LIMIT_GLOBAL_RPM", "1000"); err != nil {
		return fmt.Errorf("failed to set RATE_LIMIT_GLOBAL_RPM: %w", err)
	}
	if err := os.Setenv("RATE_LIMIT_USER_RPM", "100"); err != nil {
		return fmt.Errorf("failed to set RATE_LIMIT_USER_RPM: %w", err)
	}
	if err := os.Setenv("RATE_LIMIT_IP_RPM", "60"); err != nil {
		return fmt.Errorf("failed to set RATE_LIMIT_IP_RPM: %w", err)
	}
	if err := os.Setenv("JWT_PUBLIC_KEY", string(pubPEM)); err != nil {
		return fmt.Errorf("failed to set JWT_PUBLIC_KEY: %w", err)
	}
	if err := os.Setenv("JWT_PRIVATE_KEY", string(privPEM)); err != nil {
		return fmt.Errorf("failed to set JWT_PRIVATE_KEY: %w", err)
	}

	cfg, err := config.Load("", "", "")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}
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
		if err := testServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("failed to shutdown server: %v", err)
		}
	}

	if redisContainer != nil {
		if err := redisContainer.Terminate(ctx); err != nil {
			log.Printf("failed to terminate redis container: %v", err)
		}
	}
}

func getFreePort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, err
	}
	defer func() { _ = listener.Close() }()
	return listener.Addr().(*net.TCPAddr).Port, nil
}

func waitForServer(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}

	for time.Now().Before(deadline) {
		resp, err := client.Get(url + "/health")
		if err == nil {
			_ = resp.Body.Close()
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

func signTestServiceToken(serviceName string) string {
	now := time.Now()
	claims := jwtlib.MapClaims{
		"sub": serviceName,
		"aud": "api-gateway",
		"iss": serviceName,
		"iat": now.Unix(),
		"exp": now.Add(5 * time.Minute).Unix(),
	}
	token := jwtlib.NewWithClaims(jwtlib.SigningMethodRS256, claims)
	tokenString, _ := token.SignedString(testPrivateKey)
	return tokenString
}
