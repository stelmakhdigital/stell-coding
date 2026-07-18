package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"stell/coding-agent/internal/agent"
)

func formatResumeCommand(invokedAs, sessPath, explicitWorkspace, sessionCWD string) string {
	if sessPath == "" {
		return ""
	}
	absSess, err := filepath.Abs(sessPath)
	if err != nil {
		absSess = sessPath
	}

	parts := []string{invokedAs, "--session", shellArg(absSess)}
	if explicitWorkspace != "" {
		absWS, err := filepath.Abs(explicitWorkspace)
		if err != nil {
			absWS = explicitWorkspace
		}
		absCWD := absWS
		if sessionCWD != "" {
			if p, err := filepath.Abs(sessionCWD); err == nil {
				absCWD = p
			}
		}
		if absWS != absCWD {
			parts = append(parts, "--workspace", shellArg(absWS))
		}
	}
	return "Resume: " + strings.Join(parts, " ")
}

func shellArg(s string) string {
	if strings.IndexFunc(s, func(r rune) bool {
		switch r {
		case ' ', '\t', '\n', '"', '\\', '$', '`', '*', '?', '[', ']', '(', ')', '{', '}', '&', '|', ';', '<', '>', '!', '#', '~', '%':
			return true
		default:
			return false
		}
	}) >= 0 {
		return strconv.Quote(s)
	}
	return s
}

func printResumeHint(invokedAs string, svc *agent.Service, noSession bool, explicitWorkspace string) {
	if noSession || svc == nil || svc.SessPath == "" {
		return
	}
	if os.Getenv("STELL_NO_RESUME_HINT") != "" {
		return
	}
	_ = svc.Sessions.Save(svc.SessPath)
	if line := formatResumeCommand(invokedAs, svc.SessPath, explicitWorkspace, svc.Sessions.Header.CWD); line != "" {
		fmt.Println(line)
	}
}
