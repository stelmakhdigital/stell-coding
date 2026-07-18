package extensions

import "fmt"

func (m *Manifest) GrantKey(pkgName string) string {
	return fmt.Sprintf("%s/%s", pkgName, m.Name)
}

func (m *Manifest) needsPermissions() bool {
	return m.Permissions.Shell || m.Permissions.Network || len(m.Permissions.Paths) > 0
}
