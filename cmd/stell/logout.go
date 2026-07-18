package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/stelmakhdigital/stell-coding/internal/config"
)

func runLogout(args []string) int {
	fs := flag.NewFlagSet("logout", flag.ContinueOnError)
	_ = fs.Parse(args)
	rest := fs.Args()

	provider := ""
	if len(rest) > 0 {
		provider = strings.TrimSpace(rest[0])
	}
	globalDir, err := config.GlobalDir()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	auth, err := config.LoadAuth(globalDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if provider == "" {
		if len(auth.Providers()) == 0 {
			fmt.Fprintln(os.Stderr, "no credentials stored")
			return 1
		}
		for _, p := range auth.Providers() {
			if err := auth.Logout(p); err != nil {
				fmt.Fprintln(os.Stderr, err)
				return 1
			}
			fmt.Fprintf(os.Stderr, "removed credentials for %q\n", p)
		}
		return 0
	}
	if err := auth.Logout(provider); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Fprintf(os.Stderr, "removed credentials for %q\n", provider)
	return 0
}
