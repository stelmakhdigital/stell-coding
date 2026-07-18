package tui

import (
	"fmt"
	"strings"

	"github.com/stelmakhdigital/stell-coding/internal/extensions"
)

func (m *Model) openGrantOverlay(req extensions.GrantRequest) {
	m.grantReq = &req
	m.pushOverlayFrame(overlayFrame{
		mode: overlayGrant,
		text: renderGrantOverlay(req),
	})
}

func renderGrantOverlay(req extensions.GrantRequest) string {
	var b strings.Builder
	b.WriteString("extension permission request\n")
	fmt.Fprintf(&b, "  key: %s\n", req.Key)
	if req.Perms.Shell {
		b.WriteString("  - shell access\n")
	}
	if req.Perms.Network {
		b.WriteString("  - network access\n")
	}
	for _, p := range req.Perms.Paths {
		b.WriteString("  - path: ")
		b.WriteString(p)
		b.WriteString("\n")
	}
	b.WriteString("\ny grant · n deny · esc deny")
	return b.String()
}

func (m *Model) respondGrant(granted bool) {
	if m.grantReq == nil || m.svc.GrantBroker == nil {
		m.closeOverlay()
		return
	}
	id := m.grantReq.ID
	key := m.grantReq.Key
	_ = m.svc.GrantBroker.Respond(id, granted)
	if granted {
		m.addInfo("granted: " + key)
	} else {
		m.addInfo("denied: " + key)
	}
	m.grantReq = nil
	m.closeOverlay()
}
