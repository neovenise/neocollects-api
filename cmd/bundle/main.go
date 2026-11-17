// Command bundle exports and imports a full backup.
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

	"github.com/neovenise/neocollects-api/internal/backup"
	"github.com/neovenise/neocollects-api/internal/platform/config"
	"github.com/neovenise/neocollects-api/internal/platform/storage"
)

func main() {
	if len(os.Args) < 2 || (os.Args[1] != "export" && os.Args[1] != "import") {
		fmt.Fprintln(os.Stderr, "usage: bundle export -out FILE [-db FILE]\n       bundle import -in FILE [-db-out FILE]")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	var err error
	if os.Args[1] == "export" {
		err = runExport(ctx, os.Args[2:])
	} else {
		err = runImport(ctx, os.Args[2:])
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "bundle:", err)
		os.Exit(1)
	}
}

func openStore() (*storage.S3, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if !cfg.HasImageStorage() {
		return nil, fmt.Errorf("no image storage: set S3_ENDPOINT, S3_BUCKET, S3_ACCESS_KEY and S3_SECRET_KEY")
	}
	return storage.NewS3(storage.S3Config{
		Endpoint: cfg.S3Endpoint, Bucket: cfg.S3Bucket, AccessKey: cfg.S3AccessKey, SecretKey: cfg.S3SecretKey, UseSSL: cfg.S3UseSSL,
	})
}

func progress(verb string) func(int) {
	return func(n int) {
		if n%2000 == 0 {
			fmt.Fprintf(os.Stderr, "bundle: %s %d pictures…\n", verb, n)
		}
	}
}

// runExport writes the database and pictures into one file.
func runExport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("export", flag.ExitOnError)
	out := fs.String("out", "", "the bundle to write")
	dbPath := fs.String("db", "", "a database dump to put in it (scripts/backup-all.sh makes one)")
	_ = fs.Parse(args)
	if *out == "" {
		return fmt.Errorf("export needs -out FILE")
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	var db io.Reader
	var dbSize int64
	if *dbPath != "" {
		f, err := os.Open(*dbPath)
		if err != nil {
			return err
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			return err
		}
		db, dbSize = f, info.Size()
	}

	tmp := *out + ".partial"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	m, err := backup.Export(ctx, store, db, dbSize, f, progress("packed"))
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		_ = os.Remove(tmp)
		return err
	}
	if err := os.Rename(tmp, *out); err != nil {
		return err
	}
	fmt.Printf("bundle: %s: %d pictures, %.1f MB, database: %v\n", *out, m.Objects, float64(m.Bytes)/1e6, m.HasDatabase)
	return nil
}

// runImport restores the pictures and extracts the database dump.
func runImport(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("import", flag.ExitOnError)
	in := fs.String("in", "", "the bundle to read")
	dbOut := fs.String("db-out", "", "where to write the database dump of the bundle")
	_ = fs.Parse(args)
	if *in == "" {
		return fmt.Errorf("import needs -in FILE")
	}
	store, err := openStore()
	if err != nil {
		return err
	}
	f, err := os.Open(*in)
	if err != nil {
		return err
	}
	defer f.Close()
	var db io.Writer
	if *dbOut != "" {
		d, err := os.Create(*dbOut)
		if err != nil {
			return err
		}
		defer d.Close()
		db = d
	}
	m, err := backup.Import(ctx, f, store, db, progress("loaded"))
	if err != nil {
		return err
	}
	fmt.Printf("bundle: %d pictures loaded and checked against the manifest (%.1f MB)\n", m.Objects, float64(m.Bytes)/1e6)
	return nil
}
