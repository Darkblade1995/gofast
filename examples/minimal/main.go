package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"gofast/gofast"
)

type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginOutput struct {
	Token string `json:"token"`
}

// jwtSecret is hardcoded here only because this is a minimal,
// disposable example. Real applications must load this from an
// environment variable or secrets manager — never commit a real
// secret to source control.
var jwtSecret = []byte("example-secret-do-not-use-in-production")

func Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	claims := jwt.MapClaims{
		"sub": in.Email,
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	signed, err := token.SignedString(jwtSecret)
	if err != nil {
		return LoginOutput{}, gofast.NewBusinessError(
			gofast.ErrCodeInternal,
			http.StatusInternalServerError,
			"failed to issue token",
		)
	}

	return LoginOutput{Token: signed}, nil
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

func main() {
	router := gofast.NewRouter(
		gofast.WithAllowedOrigins("http://localhost:5173"),
	)
	router.Register("POST", "/login", gofast.Handler(Login))
	router.Register("GET", "/accounts/{id}", gofast.Handler(GetAccount).Wrap(gofast.AuthMiddleware(jwtSecret)))
	router.Register("GET", "/transactions", gofast.Handler(ListTransactions))

	router.ServeOpenAPI("/openapi.json", "GoFast Example", "1.0.0")
	router.ServeSwaggerUI("/docs", "/openapi.json")

	corsMiddleware := gofast.CORS(router)
	handler := corsMiddleware(gofast.Recovery(gofast.Logger(router)))

	server := &http.Server{
		Addr:              ":8080",
		Handler:           handler,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
	}

	log.Println("server running on :8080")
	log.Fatal(server.ListenAndServe())
}