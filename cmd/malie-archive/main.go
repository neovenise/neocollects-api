// Command malie-archive keeps a local copy of the foil data.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/neovenise/neocollects-api/internal/platform/config"
	"github.com/neovenise/neocollects-api/internal/sources/malie"
)

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "create" && os.Args[1] != "verify") {
		fmt.Fprintln(os.Stderr, "usage: malie-archive create|verify -dir FOLDER")
		os.Exit(2)
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	dir := fs.String("dir", "archive/malie", "the archive's folder")
	concurrency := fs.Int("concurrency", 6, "pictures downloaded at once (create)")
	_ = fs.Parse(os.Args[2:])

	cfg, err := loadConfig()
	if err != nil {
		fail(err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch os.Args[1] {
	case "create":
		client := malie.NewClient(cfg.MalieBaseURL, cfg.MalieLanguage)
		m, err := client.Create(ctx, *dir, malie.CreateOptions{Concurrency: *concurrency, Progress: func(done, total int) {
			fmt.Printf("pictures %d / %d\n", done, total)
		}})
		if err != nil {
			fail(fmt.Errorf("%w (run it again: what is already downloaded is kept)", err))
		}
		fmt.Printf("archive %s: %s, %d sets, %d cards, %d pictures, %.2f GB\n", *dir, m.Language, m.Sets, m.Cards, m.Pictures, float64(m.Bytes)/1e9)
	case "verify":
		checked, missing, err := malie.OpenArchive(*dir, cfg.MalieLanguage).Verify(ctx)
		if err != nil {
			fail(err)
		}
		fmt.Printf("checked %d pictures, %d missing\n", checked, len(missing))
		for i, u := range missing {
			if i == 20 {
				fmt.Printf("… and %d more\n", len(missing)-20)
				break
			}
			fmt.Println("missing:", u)
		}
		if len(missing) > 0 {
			os.Exit(1)
		}
	}
}

func loadConfig() (*config.Config, error) {
	if os.Getenv("DATABASE_URL") == "" {
		os.Setenv("DATABASE_URL", "unused")
	}
	return config.Load()
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "malie-archive:", err)
	os.Exit(1)
}
