package update

import (
	"os"
	"strings"
)

func isTruthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

// Offline сообщает, отключены ли все сетевые операции при старте.
func Offline() bool {
	return isTruthy(os.Getenv("STELL_OFFLINE"))
}

// SkipVersionCheck сообщает, отключена ли проверка версии CLI.
func SkipVersionCheck() bool {
	return Offline() || isTruthy(os.Getenv("STELL_SKIP_VERSION_CHECK"))
}

// EnableOffline выставляет STELL_OFFLINE и STELL_SKIP_VERSION_CHECK.
func EnableOffline() {
	_ = os.Setenv("STELL_OFFLINE", "1")
	_ = os.Setenv("STELL_SKIP_VERSION_CHECK", "1")
}
