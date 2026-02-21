package quokka_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jrgalyan/quokka"
)

func ExampleNew() {
	r := quokka.New()
	r.GET("/hello/:name", func(c *quokka.Context) {
		c.JSON(http.StatusOK, map[string]string{"hello": c.Param("name")})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/hello/world", nil)
	r.ServeHTTP(w, req)
	fmt.Println(w.Code)
	fmt.Println(strings.TrimSpace(w.Body.String()))
	// Output:
	// 200
	// {"hello":"world"}
}

func ExampleRouter_Group() {
	r := quokka.New()

	api := r.Group("/api")
	api.GET("/users", func(c *quokka.Context) {
		c.JSON(http.StatusOK, map[string]string{"path": "users"})
	})
	api.GET("/users/:id", func(c *quokka.Context) {
		c.JSON(http.StatusOK, map[string]string{"id": c.Param("id")})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/users/42", nil)
	r.ServeHTTP(w, req)
	fmt.Println(w.Code)
	fmt.Println(strings.TrimSpace(w.Body.String()))
	// Output:
	// 200
	// {"id":"42"}
}

func ExampleContext_JSON() {
	r := quokka.New()
	r.GET("/status", func(c *quokka.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	r.ServeHTTP(w, req)
	fmt.Println(w.Header().Get("Content-Type"))
	fmt.Println(strings.TrimSpace(w.Body.String()))
	// Output:
	// application/json; charset=utf-8
	// {"status":"ok"}
}

func ExampleContext_BindJSON() {
	type Input struct {
		Name string `json:"name"`
	}

	r := quokka.New()
	r.POST("/greet", func(c *quokka.Context) {
		var in Input
		if err := c.BindJSON(&in); err != nil {
			c.JSON(http.StatusBadRequest, quokka.ErrorResponse{Error: "bad request"})
			return
		}
		c.JSON(http.StatusOK, map[string]string{"greeting": "hello, " + in.Name})
	})

	body := strings.NewReader(`{"name":"quokka"}`)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/greet", body)
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)
	fmt.Println(w.Code)
	fmt.Println(strings.TrimSpace(w.Body.String()))
	// Output:
	// 200
	// {"greeting":"hello, quokka"}
}

func ExampleJWTAuth() {
	secret := []byte("my-secret-key")

	r := quokka.New()
	r.GET("/protected", func(c *quokka.Context) {
		claims, _ := quokka.JWTClaims(c.Context())
		c.JSON(http.StatusOK, map[string]any{"sub": claims["sub"]})
	}, quokka.JWTAuth(quokka.JWTConfig{
		Keyfunc: func(t *jwt.Token) (any, error) {
			return secret, nil
		},
	}))

	// Create a valid token for the example.
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{"sub": "user-1"})
	signed, _ := tok.SignedString(secret)

	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	req.Header.Set("Authorization", "Bearer "+signed)
	r.ServeHTTP(w, req)
	fmt.Println(w.Code)
	fmt.Println(strings.TrimSpace(w.Body.String()))
	// Output:
	// 200
	// {"sub":"user-1"}
}

func ExampleNewServer() {
	r := quokka.New()
	r.GET("/health", func(c *quokka.Context) {
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	srv := quokka.NewServer(quokka.ServerConfig{Addr: ":9090"}, r, nil)
	fmt.Println(srv.HTTP.Addr)
	// Output:
	// :9090
}
