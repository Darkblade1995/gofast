package gofast_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"gofast/gofast"
)

type ExampleLoginInput struct {
	Email string `json:"email"`
}

type ExampleLoginOutput struct {
	Token string `json:"token"`
}

func ExampleHandler() {
	login := func(ctx context.Context, in ExampleLoginInput) (ExampleLoginOutput, error) {
		return ExampleLoginOutput{Token: "token-for-" + in.Email}, nil
	}

	info := gofast.Handler(login)

	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(`{"email":"a@a.com"}`))
	rec := httptest.NewRecorder()

	info.Func(rec, req)

	var out ExampleLoginOutput
	if err := json.NewDecoder(rec.Body).Decode(&out); err != nil {
		fmt.Println("decode error:", err)
		return
	}

	fmt.Println(out.Token)
	// Output: token-for-a@a.com
}

func ExampleNewBusinessError() {
	err := gofast.NewBusinessError(gofast.ErrCodeNotFound, http.StatusNotFound, "account not found")

	fmt.Println(err.Error())
	// Output: account not found
}
