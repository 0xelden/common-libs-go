package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-errors/errors"
)

const migrationHeader = `-- Alur kerja migration: buat feature branch baru, lalu tambahkan file migrations/00x-YY-MM-DD-EPOCH-<desc>.sql
-- Nama file yang sama akan memblok merge.
-- Jangan gunakan BEGIN/COMMIT/ROLLBACK (runner sudah membungkus setiap migration di dalam transaction).
-- Jangan gunakan CREATE INDEX CONCURRENTLY (tidak bisa dijalankan di dalam transaction).
-- Jaga agar perubahan schema tetap backward-compatible dengan versi aplikasi sebelumnya selama rolling deploy.
-- PENTING: setelah sebuah migration diterapkan, file tersebut tidak boleh diedit, diubah namanya, atau dihapus
-- Karena migration lib memverifikasi hash dan keberadaan file.
-- Hanya migrate up: tulis perubahan forward-only. Tidak ada "Down".
-- Semua SQL harus idempotent: selalu asumsikan migration/statement bisa dieksekusi berulang kali pada database yang sama.
-- Eksekusi berulang tidak boleh menyebabkan error, duplikasi, overwrite yang tidak disengaja, atau kehilangan data.
-- Jangan pernah menulis migration yang berisiko menyebabkan loss of data, hindari operasi destruktif kecuali benar-benar aman
-- dan kompatibel dengan kondisi database yang mungkin sudah sebagian/seluruhnya berubah.

`

type config struct {
	dir        string
	desc       string
	flyway     bool
	flywayType string
}

type flywayType string

const (
	flywayTypeVersioned flywayType = "V"
	flywayTypeUndo      flywayType = "U"
	flywayTypeRepeat    flywayType = "R"
)

const flywayMigrationHeader = migrationHeader

func main() {
	cfg := parseFlags()
	if err := run(cfg, time.Now()); err != nil {
		//goland:noinspection GoUnhandledErrorResult
		fmt.Fprintf(os.Stderr, "gen-migrations: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() config {
	var cfg config
	flag.StringVar(&cfg.dir, "dir", "migrations", "directory to write migrations into")
	flag.StringVar(&cfg.desc, "desc", "", "migration description used in the filename")
	flag.BoolVar(&cfg.flyway, "flyway", false, "generate Flyway-style migration filenames")
	flag.StringVar(&cfg.flywayType, "flyway-type", "versioned", "Flyway migration type: versioned|v|V, undo|u|U, repeatable|r|R")
	flag.Parse()
	if strings.TrimSpace(cfg.desc) == "" && flag.NArg() > 0 {
		cfg.desc = strings.Join(flag.Args(), " ")
	}
	return cfg
}

func run(cfg config, now time.Time) error {
	desc := strings.TrimSpace(cfg.desc)
	if desc == "" {
		return errors.New("desc is required")
	}

	migrationType := flywayTypeVersioned
	if cfg.flyway {
		var err error
		migrationType, err = normalizeFlywayType(cfg.flywayType)
		if err != nil {
			return err
		}
	}

	absDir, err := filepath.Abs(cfg.dir)
	if err != nil {
		return errors.Errorf("resolve migrations dir: %v", err)
	}

	seq := 0
	if !cfg.flyway || migrationType != flywayTypeRepeat {
		seq, err = nextSequence(absDir)
		if err != nil {
			return err
		}
	}

	filename, err := buildMigrationFilename(cfg, seq, now, desc, migrationType)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(absDir, 0o755); err != nil {
		return errors.Errorf("create migrations dir: %v", err)
	}

	fullPath := filepath.Join(absDir, filename)
	file, err := os.OpenFile(fullPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return errors.Errorf("create migration file: %v", err)
	}
	defer file.Close()

	content := buildMigrationHeader(cfg.flyway) + "\n"
	if _, err := file.WriteString(content); err != nil {
		return errors.Errorf("write migration file: %v", err)
	}

	fmt.Println(fullPath)
	return nil
}

func nextSequence(dir string) (seq int, err error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return 1, nil
		}
		return 0, errors.Errorf("read migrations dir: %v", err)
	}

	maxSeq := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		seqVal, ok := parseMigrationSequence(entry.Name())
		if !ok {
			continue
		}
		if seqVal > maxSeq {
			maxSeq = seqVal
		}
	}

	return maxSeq + 1, nil
}

func buildMigrationFilename(cfg config, seq int, now time.Time, desc string, migrationType flywayType) (filename string, err error) {
	if cfg.flyway {
		return buildFlywayMigrationFilename(seq, desc, migrationType)
	}

	if seq <= 0 {
		return "", errors.New("seq must be positive")
	}

	slug, err := slugify(desc)
	if err != nil {
		return "", err
	}

	date := now.Format("06-01-02")
	epoch := now.Unix()
	filename = fmt.Sprintf("%03d-%s-%d-%s.sql", seq, date, epoch, slug)
	return filename, nil
}

func buildFlywayMigrationFilename(seq int, desc string, migrationType flywayType) (string, error) {
	slug, err := slugify(desc)
	if err != nil {
		return "", err
	}

	switch migrationType {
	case flywayTypeVersioned, flywayTypeUndo:
		if seq <= 0 {
			return "", errors.New("seq must be positive")
		}
		return fmt.Sprintf("%s%d__%s.sql", migrationType, seq, slug), nil
	case flywayTypeRepeat:
		return fmt.Sprintf("%s__%s.sql", migrationType, slug), nil
	default:
		return "", errors.Errorf("unsupported flyway type: %s", migrationType)
	}
}

func normalizeFlywayType(input string) (flywayType, error) {
	switch strings.TrimSpace(input) {
	case "versioned", "v", "V":
		return flywayTypeVersioned, nil
	case "undo", "u", "U":
		return flywayTypeUndo, nil
	case "repeatable", "r", "R":
		return flywayTypeRepeat, nil
	default:
		return "", errors.Errorf("invalid flyway type %q: use versioned|v|V, undo|u|U, or repeatable|r|R", input)
	}
}

func buildMigrationHeader(flyway bool) string {
	if flyway {
		return flywayMigrationHeader
	}
	return migrationHeader
}

func parseMigrationSequence(name string) (int, bool) {
	if len(name) >= 4 && strings.HasSuffix(name, ".sql") {
		if seqVal, err := strconv.Atoi(name[:3]); err == nil && seqVal > 0 {
			return seqVal, true
		}
	}

	if len(name) < 5 || !strings.HasSuffix(name, ".sql") {
		return 0, false
	}

	switch name[0] {
	case 'V', 'v', 'U', 'u':
	default:
		return 0, false
	}

	sepIdx := strings.Index(name, "__")
	if sepIdx <= 1 {
		return 0, false
	}

	seqVal, err := strconv.Atoi(name[1:sepIdx])
	if err != nil || seqVal <= 0 {
		return 0, false
	}

	return seqVal, true
}

func slugify(desc string) (slug string, err error) {
	trimmed := strings.TrimSpace(desc)
	if trimmed == "" {
		return "", errors.New("desc is required")
	}

	lower := strings.ToLower(trimmed)
	var b strings.Builder
	lastDash := false

	for _, r := range lower {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
			lastDash = false
		case r >= '0' && r <= '9':
			b.WriteRune(r)
			lastDash = false
		case r == ' ' || r == '_' || r == '-':
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		default:
			if b.Len() > 0 && !lastDash {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}

	slug = strings.Trim(b.String(), "-")
	if slug == "" {
		return "", errors.New("desc must include letters or digits")
	}

	return slug, nil
}
