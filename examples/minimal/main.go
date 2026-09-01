package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	goredis "github.com/redis/go-redis/v9"

	"gofast/gofast"
	memratelimit "gofast/gofast/ratelimit/memory"
	gofastredis "gofast/gofast/revocation/redis"
)

type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginOutput struct {
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
}

// jwtSecret is hardcoded here only because this is a minimal,
// disposable example. Real applications must load this from an
// environment variable or secrets manager — never commit a real
// secret to source control.
var jwtSecret = []byte("example-secret-do-not-use-in-production")

// redisClient connects to a local Redis instance. In this minimal
// example the address is hardcoded; a real application should load
// it from configuration. See ADR 0010 — revocation is optional and
// entirely opt-in; this example demonstrates it turned on.
var redisClient = goredis.NewClient(&goredis.Options{
	Addr: "localhost:6379",
})

var tokenRevoker = gofastredis.New(redisClient)

// loginRateLimiter protects /login from brute-force credential
// attempts: 1 request/second sustained, burst of 5, keyed by
// client IP (the default). See ADR 0011.
var loginRateLimiter = memratelimit.New(1, 5)

func Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	accessToken, err := gofast.IssueAccessToken(jwtSecret, in.Email, time.Hour)
	if err != nil {
		return LoginOutput{}, gofast.NewBusinessError(
			gofast.ErrCodeInternal,
			http.StatusInternalServerError,
			"failed to issue access token",
		)
	}

	refreshToken, err := gofast.IssueRefreshToken(jwtSecret, in.Email, 7*24*time.Hour)
	if err != nil {
		return LoginOutput{}, gofast.NewBusinessError(
			gofast.ErrCodeInternal,
			http.StatusInternalServerError,
			"failed to issue refresh token",
		)
	}

	return LoginOutput{Token: accessToken, RefreshToken: refreshToken}, nil
}

type GetAccountInput struct {
	ID string `path:"id"`
}

type GetAccountOutput struct {
	ID      string `json:"id"`
	Balance int64  `json:"balance"`
}

func GetAccount(ctx context.Context, in GetAccountInput) (GetAccountOutput, error) {
	if in.ID == "999" {
		return GetAccountOutput{}, gofast.NewBusinessError(
			gofast.ErrCodeNotFound,
			http.StatusNotFound,
			"account not found",
		)
	}
	return GetAccountOutput{ID: in.ID, Balance: 100000}, nil
}

type ListTransactionsInput struct {
	Page  int `query:"page,default=1"`
	Limit int `query:"limit,default=10"`
}

type ListTransactionsOutput struct {
	Page  int      `json:"page"`
	Limit int      `json:"limit"`
	Items []string `json:"items"`
}

func ListTransactions(ctx context.Context, in ListTransactionsInput) (ListTransactionsOutput, error) {
	return ListTransactionsOutput{
		Page:  in.Page,
		Limit: in.Limit,
		Items: []string{"tx-1", "tx-2"},
	}, nil
}

type LogoutOutput struct {
	Message string `json:"message"`
}

// Logout revokes the caller's current access token via
// tokenRevoker. It requires an authenticated request (registered
// behind AuthMiddleware), and reads the token's jti and expiration
// from the context claims that AuthMiddleware already populated.
// See ADR 0010.
func Logout(ctx context.Context, in struct{}) (LogoutOutput, error) {
	claims, ok := gofast.ClaimsFromContext(ctx)
	if !ok {
		return LogoutOutput{}, gofast.NewBusinessError(
			gofast.ErrCodeUnauthorized,
			http.StatusUnauthorized,
			"no authenticated session found",
		)
	}

	expiresAt := time.Unix(claims.Expires, 0)
	if err := tokenRevoker.Revoke(ctx, claims.JTI, expiresAt); err != nil {
		return LogoutOutput{}, gofast.NewBusinessError(
			gofast.ErrCodeInternal,
			http.StatusInternalServerError,
			"failed to revoke token",
		)
	}

	return LogoutOutput{Message: "logged out"}, nil
}

// EchoWS is a minimal WebSocket handler: it reads a JSON string
// message and writes it back prefixed with "echo: ". See ADR 0013.
func EchoWS(ctx context.Context, conn *websocket.Conn) error {
	for {
		var msg string
		if err := wsjson.Read(ctx, conn, &msg); err != nil {
			return nil // connection closed by client, or read error — either way, stop
		}
		if err := wsjson.Write(ctx, conn, "echo: "+msg); err != nil {
			return err
		}
	}
}

func main() {
	router := gofast.NewRouter(
		gofast.WithAllowedOrigins("http://localhost:5173"),
	)
	router.Register("POST", "/login", gofast.Handler(Login).Wrap(gofast.RateLimitMiddleware(loginRateLimiter, nil)))
	router.Register("POST", "/refresh", gofast.RefreshHandler(jwtSecret, time.Hour, tokenRevoker))
	router.Register("GET", "/accounts/{id}", gofast.Handler(GetAccount).Wrap(gofast.AuthMiddleware(jwtSecret, tokenRevoker)))
	router.Register("POST", "/logout", gofast.Handler(Logout).Wrap(gofast.AuthMiddleware(jwtSecret, tokenRevoker)))
	router.Register("GET", "/transactions", gofast.Handler(ListTransactions))
	router.RegisterWS("/ws/echo", EchoWS)

	router.ServeOpenAPI("/openapi.json", "GoFast Example", "1.0.0")
	router.ServeSwaggerUI("/docs", "/openapi.json")

	router.OnStartup(func(ctx context.Context) error {
		log.Println("checking Redis connectivity...")
		if err := redisClient.Ping(ctx).Err(); err != nil {
			return fmt.Errorf("redis unreachable at startup: %w", err)
		}
		log.Println("redis reachable")
		log.Println("server running on :8080")
		return nil
	})

	router.OnShutdown(func(ctx context.Context) error {
		log.Println("closing redis connection...")
		return redisClient.Close()
	})

	corsMiddleware := gofast.CORS(router)
	handler := corsMiddleware(gofast.Recovery(gofast.Logger(router)))

	if err := router.Run(context.Background(), ":8080", handler); err != nil {
		log.Fatal(err)
	}
}