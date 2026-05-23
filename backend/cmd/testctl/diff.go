package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

// diffRuns compares two run files and emits a markdown summary table plus a
// bug-delta breakdown. Intended to save the 15-30 minute manual synthesis
// step at the end of each agentic run.
//
//	testctl diff-runs --from run-5.json --to run-6.json
//	testctl diff-runs --from run-5.json --to run-6.json --format=plain
func diffRuns(args []string) {
	fs := flag.NewFlagSet("diff-runs", flag.ExitOnError)
	from := fs.String("from", "", "previous run JSON file (required)")
	to := fs.String("to", "", "current run JSON file (required)")
	format := fs.String("format", "markdown", "output format: markdown | plain")
	if err := fs.Parse(args); err != nil {
		log.Fatal(err)
	}
	if *from == "" || *to == "" {
		log.Fatal("--from and --to are both required")
	}

	prev, err := loadRun(*from)
	if err != nil {
		log.Fatalf("load --from: %v", err)
	}
	curr, err := loadRun(*to)
	if err != nil {
		log.Fatalf("load --to: %v", err)
	}

	prevByID := reportByPersona(prev)
	currByID := reportByPersona(curr)
	ids := sortedPersonaIDs(prevByID, currByID)

	var b strings.Builder
	writeHeader(&b, *format, prev.RunID, curr.RunID, *from, *to)
	writeScoreTable(&b, *format, ids, prevByID, currByID)
	writeBugDelta(&b, *format, prevByID, currByID)
	writeNewBugs(&b, *format, prevByID, currByID)
	writeResolvedBugs(&b, *format, prevByID, currByID)

	if _, err := os.Stdout.WriteString(b.String()); err != nil {
		log.Fatal(err)
	}
}

func writeHeader(b *strings.Builder, format, prevID, currID, fromPath, toPath string) {
	fromLabel := prevID
	if fromLabel == "" {
		fromLabel = fromPath
	}
	toLabel := currID
	if toLabel == "" {
		toLabel = toPath
	}
	switch format {
	case "markdown":
		fmt.Fprintf(b, "# Run diff: %s → %s\n\n", fromLabel, toLabel)
	default:
		fmt.Fprintf(b, "Run diff: %s -> %s\n\n", fromLabel, toLabel)
	}
}

func writeScoreTable(b *strings.Builder, format string, ids []string, prev, curr map[string]AgentReport) {
	switch format {
	case "markdown":
		b.WriteString("## Scores\n\n")
		b.WriteString("| Persona | From | To | Δ | Status |\n")
		b.WriteString("|---|---|---|---|---|\n")
	default:
		b.WriteString("SCORES\n")
		b.WriteString("persona  from  to   delta  status\n")
	}

	var improved, regressed, unchanged int
	for _, id := range ids {
		p, pok := prev[id]
		c, cok := curr[id]

		var fromScore, toScore int
		var fromStr, toStr, deltaStr string
		switch {
		case pok && cok:
			fromScore = p.Usefulness.OverallScore
			toScore = c.Usefulness.OverallScore
			fromStr = fmt.Sprintf("%d/5", fromScore)
			toStr = fmt.Sprintf("%d/5", toScore)
			delta := toScore - fromScore
			switch {
			case delta > 0:
				deltaStr = fmt.Sprintf("↑%d", delta)
				improved++
			case delta < 0:
				deltaStr = fmt.Sprintf("↓%d", -delta)
				regressed++
			default:
				deltaStr = "="
				unchanged++
			}
		case !pok && cok:
			fromStr = "—"
			toStr = fmt.Sprintf("%d/5", c.Usefulness.OverallScore)
			deltaStr = "NEW"
		case pok && !cok:
			fromStr = fmt.Sprintf("%d/5", p.Usefulness.OverallScore)
			toStr = "—"
			deltaStr = "DROPPED"
		}

		status := ""
		if cok {
			status = c.Status
		} else if pok {
			status = p.Status + " (prev only)"
		}

		name := ""
		switch {
		case cok && c.PersonaName != "":
			name = c.PersonaName
		case pok && p.PersonaName != "":
			name = p.PersonaName
		}
		label := id
		if name != "" {
			label = id + " " + name
		}

		switch format {
		case "markdown":
			fmt.Fprintf(b, "| %s | %s | %s | %s | %s |\n", label, fromStr, toStr, deltaStr, status)
		default:
			fmt.Fprintf(b, "%-30s %-6s %-6s %-8s %s\n", label, fromStr, toStr, deltaStr, status)
		}
	}

	if format == "markdown" {
		fmt.Fprintf(b, "\n**Summary:** %d improved, %d regressed, %d unchanged.\n\n", improved, regressed, unchanged)
	} else {
		fmt.Fprintf(b, "\nSummary: %d improved, %d regressed, %d unchanged.\n\n", improved, regressed, unchanged)
	}
}

func bugCounts(reports map[string]AgentReport) map[string]int {
	counts := make(map[string]int)
	for _, r := range reports {
		for _, bug := range r.Bugs {
			counts[bug.Severity]++
		}
		// ai_behavior_issues with severity contribute to the same tally
		// because agents frequently classify the same finding under either
		// bucket, and leaving them out would under-count regressions.
		for _, ai := range r.AIBehavior {
			if ai.Severity != "" {
				counts[ai.Severity]++
			}
		}
	}
	return counts
}

func writeBugDelta(b *strings.Builder, format string, prev, curr map[string]AgentReport) {
	p := bugCounts(prev)
	c := bugCounts(curr)
	sev := []string{"P0", "P1", "P2"}

	switch format {
	case "markdown":
		b.WriteString("## Bug severity totals\n\n")
		b.WriteString("| Severity | From | To | Δ |\n")
		b.WriteString("|---|---|---|---|\n")
	default:
		b.WriteString("BUG TOTALS\n")
	}
	for _, s := range sev {
		delta := c[s] - p[s]
		deltaStr := fmt.Sprintf("%+d", delta)
		if delta == 0 {
			deltaStr = "0"
		}
		switch format {
		case "markdown":
			fmt.Fprintf(b, "| %s | %d | %d | %s |\n", s, p[s], c[s], deltaStr)
		default:
			fmt.Fprintf(b, "  %s: %d -> %d (%s)\n", s, p[s], c[s], deltaStr)
		}
	}
	b.WriteString("\n")
}

// bugKey uniquely identifies a bug across runs. Since titles drift and
// descriptions are verbose, we key on persona+severity+first-60-chars of
// title so near-duplicates collapse.
func bugKey(personaID string, bug AgentBug) string {
	title := bug.Title
	if len(title) > 60 {
		title = title[:60]
	}
	return personaID + "|" + bug.Severity + "|" + strings.ToLower(title)
}

func writeNewBugs(b *strings.Builder, format string, prev, curr map[string]AgentReport) {
	prevKeys := make(map[string]struct{})
	for id, r := range prev {
		for _, bug := range r.Bugs {
			prevKeys[bugKey(id, bug)] = struct{}{}
		}
	}

	var items []string
	for id, r := range curr {
		for _, bug := range r.Bugs {
			if _, seen := prevKeys[bugKey(id, bug)]; !seen {
				items = append(items, fmt.Sprintf("- **[%s %s]** %s", id, bug.Severity, bug.Title))
			}
		}
	}
	if len(items) == 0 {
		return
	}
	switch format {
	case "markdown":
		b.WriteString("## New bugs in current run\n\n")
	default:
		b.WriteString("NEW BUGS\n")
	}
	for _, line := range items {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
}

func writeResolvedBugs(b *strings.Builder, format string, prev, curr map[string]AgentReport) {
	currKeys := make(map[string]struct{})
	for id, r := range curr {
		for _, bug := range r.Bugs {
			currKeys[bugKey(id, bug)] = struct{}{}
		}
	}

	var items []string
	for id, r := range prev {
		for _, bug := range r.Bugs {
			if _, seen := currKeys[bugKey(id, bug)]; !seen {
				items = append(items, fmt.Sprintf("- **[%s %s]** %s", id, bug.Severity, bug.Title))
			}
		}
	}
	if len(items) == 0 {
		return
	}
	switch format {
	case "markdown":
		b.WriteString("## Resolved bugs (present in --from, absent in --to)\n\n")
	default:
		b.WriteString("RESOLVED\n")
	}
	for _, line := range items {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
}
