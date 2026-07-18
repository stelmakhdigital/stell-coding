package version

// Version задаётся при сборке через -ldflags.
var Version = "dev"

// DefaultModulePath — путь модуля для go install (self-update).
const DefaultModulePath = "github.com/stell-dev/stell/cmd/stell"

// Display возвращает "stell v{Version}".
func Display() string {
	return "stell v" + Version
}
