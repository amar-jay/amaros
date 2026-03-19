package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/amar-jay/amaros/internal/memory"
	"github.com/amar-jay/amaros/pkg/config"
)

// ANSI Color Codes
const (
	Reset  = "\033[0m"
	Red    = "\033[31m"
	Green  = "\033[32m"
	Yellow = "\033[33m"
	Cyan   = "\033[36m"
	Gray   = "\033[90m"
)

var seeds = []struct {
	key   string
	value string
}{
	{"technical debt: oauth2 token refresh logic in auth middleware uses deprecated library", "priority=low;ref=JIRA-402"},
	{"bug report: oauth2 token refresh fails when system clock drifts by 5 minutes", "severity=critical;fix=use_ntp_sync"},
	{"architecture decision: oauth2 token storage moved from redis to postgres", "author=lead_dev;reason=consistency"},
	{"compliance requirement: log all middleware request headers except Authorization", "legal=strict;pii=true"},
	{"performance bottleneck: middleware logging creates high disk I/O", "fix=async_logging;sampling=10%"},
}

func main() {
	conf := config.Get()
	if conf == nil {
		fatal(errors.New("config was not loaded"))
	}

	baseDir := filepath.Join(conf.Memory.RootDir, "examples", "complex_agent_memory")
	os.RemoveAll(baseDir)
	os.MkdirAll(baseDir, 0o750)

	fmt.Printf("%s=== High-Precision Semantic Retrieval Console ===%s\n", Cyan, Reset)

	if err := runHotExample(baseDir); err != nil {
		fatal(err)
	}
}

func runHotExample(baseDir string) error {
	hot, err := memory.NewHotStore(baseDir)
	if err != nil {
		return err
	}
	defer hot.Close()

	// 1. INGESTION
	step("1) Ingesting high-overlap memories (Auth & Middleware)")
	seeds := []struct {
		key   string
		value string
	}{
		{"technical debt: oauth2 token refresh logic in auth middleware uses deprecated library", "priority=low;ref=JIRA-402"},
		{"bug report: oauth2 token refresh fails when system clock drifts by 5 minutes", "severity=critical;fix=use_ntp_sync"},
		{"architecture decision: oauth2 token storage moved from redis to postgres", "author=lead_dev;reason=consistency"},
		{"compliance requirement: log all middleware request headers except Authorization", "legal=strict;pii=true"},
		{"performance bottleneck: middleware logging creates high disk I/O", "fix=async_logging;sampling=10%"},
	}

	for _, seed := range seeds {
		if err := hot.Set(seed.key, []byte(seed.value)); err != nil {
			return err
		}
		fmt.Printf("%s[Stored]%s %s\n", Gray, Reset, seed.key)
	}

	// 2. RETRIEVAL TESTS

	step("2) Distinguishing BUG from DEBT")
	q1 := "is there an active failure or error in the oauth2 refresh process?"
	e1, _ := hot.Get(q1)
	printMatch(q1, e1)

	step("3) Legal vs Performance")
	q2 := "gdpr rules and privacy restrictions for logging middleware"
	e2, _ := hot.Get(q2)
	printMatch(q2, e2)

	step("4) Infrastructure History")
	q3 := "migration of tokens from redis to database for reliability"
	e3, _ := hot.Get(q3)
	printMatch(q3, e3)

	// 3. DELETION TEST
	step("5) Deleting a memory and verifying it's gone")
	delKey := seeds[0].key
	if err := hot.Delete(delKey); err != nil {
		return err
	}
	fmt.Printf("%s[Deleted]%s %s\n", Red, Reset, delKey)

	e4, _ := hot.Get(delKey)
	printMatch(delKey, e4)

	time.Sleep(2 * time.Second) // Ensure timestamps differ for update test
	// 4. UPDATING TEST
	step("6) Updating an existing memory and verifying the update")
	updateKey := seeds[1].key
	newValue := "severity=critical;fix=use_ntp_sync;status=in_progress"
	if err := hot.Set(updateKey, []byte(newValue)); err != nil {
		return err
	}
	fmt.Printf("%s[Updated]%s %s\n", Green, Reset, updateKey)

	e5, _ := hot.Get(updateKey)
	printMatch(updateKey, e5)

	return nil
}

func step(label string) {
	fmt.Printf("\n%s--- %s ---%s\n", Cyan, label, Reset)
}

func printMatch(query string, e *memory.Entry) {
	fmt.Printf("%s[Query]:%s  %s\n", Yellow, Reset, query)
	if e == nil {
		fmt.Printf("%s[Result]:%s %sNOT FOUND%s\n", Green, Reset, Red, Reset)
		return
	}
	fmt.Printf("%s[Result]:%s %s%s%s\n", Green, Reset, Green, e.Key, Reset)
	fmt.Printf("%s[Data]:%s   %s\n", Gray, Reset, string(e.Value))
	fmt.Printf("%s[Metadata]:%s CreatedAt=%s UpdatedAt=%s\n", Gray, Reset, e.CreatedAt.Format(time.RFC3339), e.UpdatedAt.Format(time.RFC3339))
}

func fatal(err error) {
	fmt.Fprintf(os.Stderr, "%sError: %v%s\n", Red, err, Reset)
	os.Exit(1)
}
