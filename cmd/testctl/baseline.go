package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// baselineCompare diffs a run against the per-persona baselines directory
// and emits a markdown regression report. A "regression" is a persona
// whose overall_score dropped, or whose bug count rose, versus the
// committed baseline file for that persona.
//
//	testctl baseline-compare --baselines tests/agentic/baselines --run run-6.json
//	testctl baseline-compare --baselines tests/agentic/baselines --run-dir tmp/run-6/
func baselineCompare(args []string) {
	fs := flag.NewFlagSet("baseline-compare", flag.ExitOnError)
	baselinesDir := fs.String("baselines", "tests/agentic/baselines", "directory containing <persona>.json baseline files")
	runFile := fs.String("run", "", "single run JSON file containing {reports: [...]}")
	runDir := fs.String("run-dir", "", "directory of per-persona JSON report files")
	format := fs.String("format", "markdown", "output format: markdown | plain")
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	if (*runFile == "") == (*runDir == "") {
		log.Fatal("exactly one of --run or --run-dir must be set")
	}

	baselines, err := loadBaselines(*baselinesDir)
	if err != nil {
		log.Fatalf("load baselines: %v", err)
	}

	var current map[string]AgentReport
	if *runFile != "" {
		run, err := loadRun(*runFile)
		if err != nil {
			log.Fatalf("load --run: %v", err)
		}
		current = reportByPersona(run)
	} else {
		current, err = loadReportsDir(*runDir)
		if err != nil {
			log.Fatalf("load --run-dir: %v", err)
		}
	}

	ids := sortedPersonaIDs(baselines, current)
	var b strings.Builder
	switch *format {
	case "markdown":
		b.WriteString("# Baseline comparison\n\n")
		b.WriteString("| Persona | Baseline | Current | Δ | Bug count Δ | Verdict |\n")
		b.WriteString("|---|---|---|---|---|---|\n")
	default:
		b.WriteString("BASELINE COMPARE\n")
	}

	var regressed, improved, unchanged, newPersonas, missing int

	for _, id := range ids {
		base, baseOK := baselines[id]
		curr, currOK := current[id]

		var baseScore, currScore int
		var baseBugs, currBugs int
		verdict := ""

		switch {
		case baseOK && currOK:
			baseScore = base.Usefulness.OverallScore
			currScore = curr.Usefulness.OverallScore
			baseBugs = len(base.Bugs)
			currBugs = len(curr.Bugs)
			switch {
			case currScore < baseScore || currBugs > baseBugs:
				verdict = "**REGRESSED**"
				regressed++
			case currScore > baseScore || currBugs < baseBugs:
				verdict = "improved"
				improved++
			default:
				verdict = "unchanged"
				unchanged++
			}
		case !baseOK && currOK:
			currScore = curr.Usefulness.OverallScore
			currBugs = len(curr.Bugs)
			verdict = "NEW (no baseline)"
			newPersonas++
		case baseOK && !currOK:
			baseScore = base.Usefulness.OverallScore
			baseBugs = len(base.Bugs)
			verdict = "**MISSING** (persona did not run)"
			missing++
		}

		baseStr := "—"
		if baseOK {
			baseStr = fmt.Sprintf("%d/5", baseScore)
		}
		currStr := "—"
		if currOK {
			currStr = fmt.Sprintf("%d/5", currScore)
		}
		scoreDelta := ""
		if baseOK && currOK {
			d := currScore - baseScore
			switch {
			case d > 0:
				scoreDelta = fmt.Sprintf("↑%d", d)
			case d < 0:
				scoreDelta = fmt.Sprintf("↓%d", -d)
			default:
				scoreDelta = "="
			}
		}
		bugDelta := ""
		if baseOK && currOK {
			d := currBugs - baseBugs
			switch {
			case d > 0:
				bugDelta = fmt.Sprintf("+%d", d)
			case d < 0:
				bugDelta = fmt.Sprintf("%d", d)
			default:
				bugDelta = "0"
			}
		}

		switch *format {
		case "markdown":
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %s | %s |\n", id, baseStr, currStr, scoreDelta, bugDelta, verdict)
		default:
			fmt.Fprintf(&b, "%-6s base=%s  curr=%s  %s  bugs=%s  %s\n", id, baseStr, currStr, scoreDelta, bugDelta, verdict)
		}
	}

	if *format == "markdown" {
		fmt.Fprintf(&b, "\n**Summary:** %d regressed, %d improved, %d unchanged, %d new, %d missing.\n",
			regressed, improved, unchanged, newPersonas, missing)
	} else {
		fmt.Fprintf(&b, "\nSummary: %d regressed, %d improved, %d unchanged, %d new, %d missing.\n",
			regressed, improved, unchanged, newPersonas, missing)
	}

	if _, err := os.Stdout.WriteString(b.String()); err != nil {
		log.Fatal(err)
	}

	// Exit 1 on regression so CI can gate on this. Go's flag package
	// already reserves exit code 2 for usage errors, so 1 keeps the
	// semantics distinct and conventional.
	if regressed > 0 {
		os.Exit(1)
	}
}

// loadBaselines reads every *.json file in the baselines directory and
// returns a persona_id → report map. Files whose name does not match a
// persona ID pattern are skipped with a warning.
func loadBaselines(dir string) (map[string]AgentReport, error) {
	out := make(map[string]AgentReport)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil // empty baselines dir is acceptable (first run)
		}
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", e.Name(), err)
		}
		var rep AgentReport
		if err := json.Unmarshal(data, &rep); err != nil {
			fmt.Fprintf(os.Stderr, "warning: baseline %s is not a valid report: %v\n", e.Name(), err)
			continue
		}
		id := rep.PersonaID
		if id == "" {
			// Fall back to filename stem.
			id = strings.TrimSuffix(e.Name(), ".json")
		}
		out[id] = rep
	}
	return out, nil
}

// loadReportsDir reads a directory of per-persona report files. Each file
// is expected to contain a single AgentReport (not a wrapped Run).
func loadReportsDir(dir string) (map[string]AgentReport, error) {
	out := make(map[string]AgentReport)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		var rep AgentReport
		if err := json.Unmarshal(data, &rep); err != nil {
			return nil, fmt.Errorf("%s: %w", e.Name(), err)
		}
		id := rep.PersonaID
		if id == "" {
			id = strings.TrimSuffix(e.Name(), ".json")
		}
		out[id] = rep
	}
	return out, nil
}
