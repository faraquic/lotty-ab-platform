package main

import (
	"fmt"
	"os"

	"github.com/alexedwards/argon2id"
)

func main() {
	if len(os.Args) != 2 || os.Args[1] == "" {
		fmt.Fprintln(os.Stderr, "usage: go run ./tools/hash-password <password>")
		os.Exit(2)
	}

	hash, err := argon2id.CreateHash(os.Args[1], argon2id.DefaultParams)
	if err != nil {
		fmt.Fprintf(os.Stderr, "create password hash: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(hash)
}
