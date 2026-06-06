package main

import (
	"flag"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/GoMudEngine/GoMud-Module-Playtest-Harness/internal/session"
)

func main() {
	target := flag.String("target", "", "host:port of the MUD AI port (overrides manifest)")
	user := flag.String("user", "", "test account username (overrides manifest)")
	pass := flag.String("password", "", "test account password (overrides manifest)")
	manifestPath := flag.String("manifest", "", "path to a run manifest YAML")
	flag.Parse()

	var m session.Manifest
	if *manifestPath != "" {
		b, err := os.ReadFile(*manifestPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "mudagent: read manifest: %v\n", err)
			os.Exit(2)
		}
		if m, err = session.ParseManifest(b); err != nil {
			fmt.Fprintf(os.Stderr, "mudagent: parse manifest: %v\n", err)
			os.Exit(2)
		}
	}
	if *target != "" {
		m.Target = *target
	}
	if *user != "" {
		m.User = *user
	}
	if *pass != "" {
		m.Password = *pass
	}
	if m.Target == "" || m.User == "" {
		fmt.Fprintln(os.Stderr, "mudagent: --target and --user (or a manifest) are required")
		os.Exit(2)
	}

	conn, err := net.DialTimeout("tcp", m.Target, 10*time.Second)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mudagent: dial %s: %v\n", m.Target, err)
		os.Exit(1)
	}
	defer conn.Close()

	if err := session.Run(conn, os.Stdin, os.Stdout, session.Config{User: m.User, Pass: m.Password}); err != nil {
		fmt.Fprintf(os.Stderr, "mudagent: %v\n", err)
		os.Exit(1)
	}
}
