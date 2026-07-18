package agent

import (
	"fmt"
	"io"
	"os"
)

// sessionSaveErrOut — куда писать ошибки сохранения сессии (тесты подменяют).
var sessionSaveErrOut io.Writer = os.Stderr

func reportSessionSaveError(path string, err error) {
	if err == nil {
		return
	}
	_, _ = fmt.Fprintf(sessionSaveErrOut, "stell: failed to save session %s: %v\n", path, err)
}
