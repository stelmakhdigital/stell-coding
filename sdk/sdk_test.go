package sdk_test

import (
	"testing"

	"stell/coding-agent/sdk"
)

func TestOptionsZeroWorkspaceDefaults(t *testing.T) {
	// CreateSession с явными пустыми Options не должен паниковать.
	opts := sdk.Options{NoTools: "all", Workspace: t.TempDir()}
	sess, err := sdk.CreateSessionOpts(opts)
	if err != nil {
		// Глобальные или пустые models зависят от окружения; оба варианта допустимы.
		t.Log(err)
		return
	}
	if sess == nil || sess.Service == nil {
		t.Fatal("nil session")
	}
	_ = sess.Model()
}
