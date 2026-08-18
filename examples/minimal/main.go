package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"gofast/gofast"
)

type LoginInput struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=8"`
}

type LoginOutput struct {
	Token string `json:"token"`
}

func Login(ctx context.Context, in LoginInput) (LoginOutput, error) {
	return LoginOutput{Token: "fake-token-for-" + in.Email}, nil
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
	router.Register("GET", "/accounts/{id}", gofast.Handler(GetAccount))
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
