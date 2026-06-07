// Package main implements the ptorch CLI, a thin wrapper around
// internal/scenario and internal/blackboard for use by the multi-agent conductor
// and per-agent runner scripts.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/GoMudEngine/GoMud-Module-Playtest-Harness/internal/blackboard"
	"github.com/GoMudEngine/GoMud-Module-Playtest-Harness/internal/scenario"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) < 1 {
		fmt.Fprintln(stderr, "usage: ptorch <scenario|bb> ...")
		return 2
	}
	switch args[0] {
	case "scenario":
		return runScenario(args[1:], stdout, stderr)
	case "bb":
		return runBB(args[1:], stdout, stderr)
	default:
		fmt.Fprintf(stderr, "unknown command %q\n", args[0])
		return 2
	}
}

func loadScenario(path string, stderr io.Writer) (scenario.Scenario, bool) {
	b, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(stderr, "read %s: %v\n", path, err)
		return scenario.Scenario{}, false
	}
	s, err := scenario.Parse(b)
	if err != nil {
		fmt.Fprintf(stderr, "parse %s: %v\n", path, err)
		return scenario.Scenario{}, false
	}
	return s, true
}

func runScenario(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: ptorch scenario <validate|plan> <file>")
		return 2
	}
	sub, path := args[0], args[1]
	s, ok := loadScenario(path, stderr)
	if !ok {
		return 1
	}
	if err := s.Validate(); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}
	switch sub {
	case "validate":
		fmt.Fprintf(stdout, "OK: %q (%s, %d agents)\n", s.Name, s.Mode, len(s.Roster))
		for _, w := range s.Warnings() {
			fmt.Fprintln(stdout, "WARNING:", w)
		}
		return 0
	case "plan":
		type rosterOut struct {
			ID     string `json:"id"`
			Role   string `json:"role"`
			Target string `json:"target"`
		}
		out := struct {
			Name           string            `json:"name"`
			Mode           string            `json:"mode"`
			Summary        string            `json:"summary"`
			MaxConnections int               `json:"max_connections"`
			Roster         []rosterOut       `json:"roster"`
			GroupGoals     []scenario.Goal   `json:"group_goals"`
			Requires       scenario.Requires `json:"requires"`
			Warnings       []string          `json:"warnings"`
		}{
			Name: s.Name, Mode: s.Mode, Summary: s.Summary,
			MaxConnections: s.MaxConnections(),
			GroupGoals:     s.GroupGoals, Requires: s.Requires,
			Warnings: s.Warnings(),
		}
		for _, r := range s.Roster {
			out.Roster = append(out.Roster, rosterOut{r.ID, r.Role, r.Target})
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown scenario subcommand %q\n", sub)
		return 2
	}
}

func runBB(args []string, stdout, stderr io.Writer) int {
	if len(args) < 2 {
		fmt.Fprintln(stderr, "usage: ptorch bb <init|ready|allready|phase|signal|finding|dump> <path> [flags]")
		return 2
	}
	sub, path := args[0], args[1]
	fs := flag.NewFlagSet("bb "+sub, flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		runName = fs.String("run", "", "run name")
		ids     = fs.String("ids", "", "comma-separated agent ids")
		id      = fs.String("id", "", "agent id")
		setPh   = fs.String("set", "", "phase to set")
		name    = fs.String("name", "", "signal name")
		round   = fs.Int("round", 0, "beacon round")
		agent   = fs.String("agent", "", "finding agent id")
		ftype   = fs.String("type", "", "finding type")
		title   = fs.String("title", "", "finding title")
	)
	if err := fs.Parse(args[2:]); err != nil {
		return 2
	}

	switch sub {
	case "init":
		var list []string
		if *ids != "" {
			list = strings.Split(*ids, ",")
		}
		if err := blackboard.Init(path, *runName, list); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "ready":
		if *id == "" {
			fmt.Fprintln(stderr, "bb ready: --id is required")
			return 2
		}
		if err := blackboard.SetReady(path, *id); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "allready":
		ok, err := blackboard.AllReady(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		if !ok {
			return 3 // not-all-ready: distinct exit code for shell branching
		}
		return 0
	case "phase":
		if *setPh != "" {
			if err := blackboard.SetPhase(path, *setPh); err != nil {
				fmt.Fprintln(stderr, err)
				return 1
			}
			return 0
		}
		p, err := blackboard.Phase(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		fmt.Fprintln(stdout, p)
		return 0
	case "signal":
		if *name == "" {
			fmt.Fprintln(stderr, "bb signal: --name is required")
			return 2
		}
		if err := blackboard.Signal(path, *name, *round); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "finding":
		if *agent == "" || *title == "" {
			fmt.Fprintln(stderr, "bb finding: --agent and --title are required")
			return 2
		}
		f := blackboard.Finding{Agent: *agent, Type: *ftype, Title: *title, Round: *round}
		if err := blackboard.AddFinding(path, f); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	case "dump":
		bd, err := blackboard.Load(path)
		if err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		enc := json.NewEncoder(stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(bd); err != nil {
			fmt.Fprintln(stderr, err)
			return 1
		}
		return 0
	default:
		fmt.Fprintf(stderr, "unknown bb subcommand %q\n", sub)
		return 2
	}
}
