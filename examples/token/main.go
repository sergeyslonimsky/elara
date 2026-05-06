package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/casbin/casbin/v2"
	"github.com/casbin/casbin/v2/model"
	"github.com/golang-jwt/jwt/v5"
	"go.etcd.io/bbolt"
	"go.etcd.io/etcd/api/v3/etcdserverpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type Claims struct {
	Email string `json:"email"`
	Name  string `json:"name"`
	jwt.RegisteredClaims
}

const defaultSecret = "super-secret-elara-session-key-32b"

const (
	minArgs  = 2
	dayHours = 24
	dbMode   = 0o600
)

func main() {
	if len(os.Args) < minArgs {
		fmt.Println("Usage: go run . [gen-tokens|check-db|test-casbin|test-grpc]")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "gen-tokens":
		genTokens()
	case "check-db":
		checkDB()
	case "test-casbin":
		testCasbin()
	case "test-grpc":
		testGRPC()
	default:
		fmt.Printf("Unknown command: %s\n", os.Args[1])
		os.Exit(1)
	}
}

func genTokens() {
	secret := []byte(defaultSecret)
	users := []struct {
		Email string
		Name  string
	}{
		{"alice@example.com", "Alice Admin"},
		{"bob@example.com", "Bob Writer"},
		{"carol@example.com", "Carol Reader"},
		{"eve@example.com", "Eve Reader"},
		{"dave@example.com", "Dave NoAccess"},
		{"frank@example.com", "Frank Writer"},
		{"grace@example.com", "Grace WriterReader"},
	}

	fmt.Println("Generated JWT Sessions (for .env or tokens.sh):")
	for _, u := range users {
		claims := Claims{
			Email: u.Email,
			Name:  u.Name,
			RegisteredClaims: jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(dayHours * time.Hour)),
			},
		}
		token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
		signed, _ := token.SignedString(secret)
		fmt.Printf("export %s_SESSION=\"elara_session=%s\"\n", stringsToUpper(u.Email), signed)
	}
}

func stringsToUpper(s string) string {
	// Simple helper to convert email to ENV-like prefix
	for i := range len(s) {
		if s[i] == '@' {
			return stringsToTitle(s[:i])
		}
	}

	return s
}

func stringsToTitle(s string) string {
	res := ""
	var resSb92 strings.Builder
	for _, r := range s {
		if r >= 'a' && r <= 'z' {
			resSb92.WriteRune(r - 'a' + 'A')
		} else {
			resSb92.WriteRune(r)
		}
	}
	res += resSb92.String()

	return res
}

func checkDB() {
	db, err := bbolt.Open("data/elara.db", dbMode, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		log.Fatalf("Could not open DB (maybe server is running?): %v", err)
	}
	defer func() {
		_ = db.Close()
	}()

	_ = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte("auth_policies"))
		if b == nil {
			fmt.Println("No auth_policies bucket found")

			return nil
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			fmt.Printf("%s: %s\n", k, v)
		}

		return nil
	})
}

func testCasbin() {
	m, _ := model.NewModelFromString(`
[request_definition]
r = sub, dom, obj, act
[policy_definition]
p = sub, dom, obj, act
[role_definition]
g = _, _, _
[policy_effect]
e = some(where (p.eft == allow))
[matchers]
m = (g(r.sub, p.sub, r.dom) || g(r.sub, p.sub, "*")) && (r.dom == p.dom || p.dom == "*") && keyMatch(r.obj, p.obj) && (r.act == p.act || p.act == "*")
`)
	e, _ := casbin.NewEnforcer(m)
	_, _ = e.AddPolicy("admin", "*", "*", "*")
	_, _ = e.AddGroupingPolicy("alice@example.com", "admin", "*")

	ok, err := e.Enforce("alice@example.com", "*", "user", "read")
	fmt.Printf("Casbin Enforce (alice, *, user, read): ok=%v, err=%v\n", ok, err)
}

func testGRPC() {
	// Note: tokens must be fresh and match what's in the DB
	// This is a template, actual tokens should be passed or fetched.
	fmt.Println("Starting gRPC Auth Test...")

	// You might want to update this token from 'gen-tokens' or 'ListTokens' API
	authToken := os.Getenv("BOB_TOKEN")
	if authToken == "" {
		authToken = "elara_DDSNETNPOAYM2MvcJIl2NMGz8OrFfYtm942azCExO8Q" //nolint:gosec // example fallback token for testing
		fmt.Printf("Using fallback token: %s\n", authToken)
	}

	conn, err := grpc.NewClient("localhost:2379", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer func() {
		_ = conn.Close()
	}()

	client := etcdserverpb.NewKVClient(conn)
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+authToken)

	// Range prod
	resp, err := client.Range(ctx, &etcdserverpb.RangeRequest{Key: []byte("/prod/db/host")})
	if err != nil {
		fmt.Printf("GET /prod/db/host: ERROR: %v\n", err)
	} else {
		fmt.Printf("GET /prod/db/host: SUCCESS, count=%d\n", resp.Count)
	}

	// Put prod
	_, err = client.Put(ctx, &etcdserverpb.PutRequest{Key: []byte("/prod/app/test"), Value: []byte("test")})
	if err != nil {
		fmt.Printf("PUT /prod/app/test: ERROR: %v (Expected if reader)\n", err)
	} else {
		fmt.Printf("PUT /prod/app/test: SUCCESS\n")
	}

	// Range dev
	resp, err = client.Range(ctx, &etcdserverpb.RangeRequest{Key: []byte("/dev/app/debug")})
	if err != nil {
		fmt.Printf("GET /dev/app/debug: ERROR: %v (Expected 403)\n", err)
	} else {
		fmt.Printf("GET /dev/app/debug: SUCCESS, count=%d\n", resp.Count)
	}
}
