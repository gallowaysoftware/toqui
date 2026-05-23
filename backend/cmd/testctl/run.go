package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

// findRepoRoot walks up from the current working directory looking for
// a go.mod file, so testctl commands invoked from any subdirectory
// still resolve skill/persona/schema paths relative to the repo root.
func findRepoRoot() (string, error) {
	dir, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("no go.mod found in any parent directory")
		}
		dir = parent
	}
}

// runPersonaPrompt is emitted when someone runs
// `testctl run-persona --id R-02` — it's the instruction block a
// human or an orchestrating Claude Code session should copy into a
// subagent Task to replay a single persona against the running server.
// Nothing in this command actually drives the API; it's purely a
// prompt-builder so the single-persona replay loop is fast and
// repeatable without having to hand-craft the instructions each time.
//
//	testctl run-persona --id R-02
//	testctl run-persona --id N-13 --token "eyJ..." --host localhost:8090
func runPersonaPrompt(args []string) {
	fs := flag.NewFlagSet("run-persona", flag.ExitOnError)
	id := fs.String("id", "", "persona ID, e.g. R-02 or N-13 (required)")
	token := fs.String("token", "$TOKEN", "JWT token (default: literal $TOKEN placeholder)")
	host := fs.String("host", "localhost:8090", "backend host:port")
	expectedEmail := fs.String("expected-email", "", "optional expected email for identity assertion")
	runID := fs.String("run-id", "", "optional run_id to stamp on the emitted report")
	personasDir := fs.String("personas", "tests/agentic/personas", "directory containing persona files (relative to repo root or absolute)")
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	if *id == "" {
		log.Fatal("--id is required (e.g. R-02 or N-13)")
	}

	absRepoRoot, err := findRepoRoot()
	if err != nil {
		log.Fatalf("find repo root: %v", err)
	}

	// Resolve personasDir relative to the repo root if it's a relative
	// path, so `testctl run-persona` works from any subdirectory.
	resolvedPersonasDir := *personasDir
	if !filepath.IsAbs(resolvedPersonasDir) {
		resolvedPersonasDir = filepath.Join(absRepoRoot, resolvedPersonasDir)
	}

	file, err := findPersonaFile(resolvedPersonasDir, *id)
	if err != nil {
		log.Fatalf("resolve persona %s: %v", *id, err)
	}

	var b strings.Builder
	fmt.Fprintf(&b, "You are running an agentic test of the Toqui backend as persona %s.\n\n", *id)
	fmt.Fprintf(&b, "REQUIRED READING:\n")
	fmt.Fprintf(&b, "1. %s/.claude/skills/agentic-test/SKILL.md — the test framework reference.\n", absRepoRoot)
	fmt.Fprintf(&b, "2. %s — this persona's specific instructions.\n", file)
	fmt.Fprintf(&b, "3. %s/tests/agentic/report-schema.json — the output schema your final report MUST validate against.\n\n", absRepoRoot)

	fmt.Fprintf(&b, "Your persona_id: %s (this MUST appear in your json-report block).\n", *id)
	fmt.Fprintf(&b, "Your token: %s\n", *token)
	fmt.Fprintf(&b, "Host: %s\n", *host)
	if *expectedEmail != "" {
		fmt.Fprintf(&b, "Expected email for identity assertion: %s\n", *expectedEmail)
	}
	if *runID != "" {
		fmt.Fprintf(&b, "Run ID: %s (include in report.run_id)\n", *runID)
	}
	fmt.Fprintln(&b)

	fmt.Fprintf(&b, "Environment setup:\n```bash\n")
	fmt.Fprintf(&b, "export PATH=\"/opt/homebrew/bin:$PATH\"\n")
	fmt.Fprintf(&b, "cd %s\n", absRepoRoot)
	fmt.Fprintf(&b, "export TOKEN=\"%s\"\n", *token)
	fmt.Fprintf(&b, "export HOST=\"%s\"\n", *host)
	fmt.Fprintf(&b, "export ORIGIN=\"http://localhost:3000\"\n")
	fmt.Fprintf(&b, "```\n\n")

	fmt.Fprintf(&b, "STEP 1 IS MANDATORY: call GetCurrentUser and assert the returned email ")
	if *expectedEmail != "" {
		fmt.Fprintf(&b, "matches %q. If it does not, abort immediately with an ABORTED_IDENTITY_MISMATCH report.\n\n", *expectedEmail)
	} else {
		fmt.Fprintf(&b, "is non-empty. Record the user details for your report.\n\n")
	}

	fmt.Fprintf(&b, "Follow the persona file's test flow. Write each API response to /tmp/%s-step-N.json as you go.\n\n", strings.ToLower(*id))
	fmt.Fprintf(&b, "Return EXACTLY one ```json-report``` code block at the end, validating against report-schema.json. Use persona_id=%q.\n\n", *id)
	fmt.Fprintf(&b, "DO NOT manage infrastructure (no docker, make run, pkill, etc.). Just test the API.\n")

	if _, err := os.Stdout.WriteString(b.String()); err != nil {
		log.Fatal(err)
	}
}

// findPersonaFile looks up a persona file by ID in a directory.
// Personas are named like "R-02-family-regression.md" — we do a
// prefix match on the ID.
func findPersonaFile(dir, id string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	prefix := id + "-"
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix) && strings.HasSuffix(e.Name(), ".md") {
			abs, err := filepath.Abs(filepath.Join(dir, e.Name()))
			if err != nil {
				return "", err
			}
			return abs, nil
		}
	}
	return "", fmt.Errorf("no persona file starting with %q in %s", prefix, dir)
}
