// Command admin sets an account's password.
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/neovenise/neocollects-api/internal/auth"
	"github.com/neovenise/neocollects-api/internal/platform/config"
	"github.com/neovenise/neocollects-api/internal/platform/database"
)

func main() {
	if len(os.Args) != 3 || os.Args[1] != "set-password" {
		fmt.Fprintln(os.Stderr, "usage: echo \"new password\" | admin set-password USERNAME")
		os.Exit(2)
	}
	if err := run(context.Background(), os.Args[2]); err != nil {
		fmt.Fprintln(os.Stderr, "admin:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, username string) error {
	// Read the new password from standard input.
	password, err := bufio.NewReader(os.Stdin).ReadString('\n')
	password = strings.TrimRight(password, "\r\n")
	if err != nil && password == "" {
		return fmt.Errorf("no password on the standard input")
	}

	cfg, err := config.Load()
	if err != nil {
		return err
	}
	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	service, err := auth.NewService(auth.NewRepository(pool), auth.DefaultHasher, auth.DefaultConfig(""))
	if err != nil {
		return err
	}
	// Replace it and sign every device out.
	if err := service.ResetPassword(ctx, username, password); err != nil {
		return err
	}
	fmt.Printf("the password of %s was replaced; every session is signed out\n", strings.TrimSpace(username))
	return nil
}
