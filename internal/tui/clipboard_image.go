package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

func readClipboardImageBytes() ([]byte, string, error) {
	switch runtime.GOOS {
	case "darwin":
		return readClipboardImageDarwin()
	case "linux":
		return readClipboardImageLinux()
	case "windows":
		return readClipboardImageWindows()
	default:
		return nil, "", nil
	}
}

func readClipboardImageDarwin() ([]byte, string, error) {
	if data, mime, err := readClipboardImageDarwinPNGpaste(); err == nil && len(data) > 0 {
		return data, mime, nil
	}
	if data, mime, err := readClipboardImageDarwinJXA(); err == nil && len(data) > 0 {
		return data, mime, nil
	}
	return readClipboardImageDarwinOsascript()
}

func readClipboardImageDarwinPNGpaste() ([]byte, string, error) {
	if _, err := exec.LookPath("pngpaste"); err != nil {
		return nil, "", err
	}
	f, err := os.CreateTemp("", "stell-clip-*.png")
	if err != nil {
		return nil, "", err
	}
	path := f.Name()
	_ = f.Close()
	defer os.Remove(path)
	if err := exec.Command("pngpaste", path).Run(); err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		return nil, "", nil
	}
	return data, "image/png", nil
}

func readClipboardImageDarwinJXA() ([]byte, string, error) {
	if _, err := exec.LookPath("osascript"); err != nil {
		return nil, "", err
	}
	f, err := os.CreateTemp("", "stell-clip-*")
	if err != nil {
		return nil, "", err
	}
	path := f.Name()
	_ = f.Close()
	defer os.Remove(path)
	script := fmt.Sprintf(`function run() {
ObjC.import('AppKit');
var pb = $.NSPasteboard.generalPasteboard;
var types = ['public.png', 'Apple PNG pasteboard type', 'public.tiff', 'public.jpeg'];
for (var i = 0; i < types.length; i++) {
  var data = pb.dataForType(types[i]);
  if (data && data.length > 0) {
    data.writeToFileAtomically($('%s'), true);
    return 'ok';
  }
}
}`, path)
	if err := exec.Command("osascript", "-l", "JavaScript", "-e", script).Run(); err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, "", nil
	}
	ext := strings.ToLower(filepath.Ext(path))
	if ext == ".png" || len(data) >= 8 && string(data[:8]) == "\x89PNG\r\n\x1a\n" {
		return data, "image/png", nil
	}
	if png, err := convertImageBytesToPNG(data); err == nil && len(png) > 0 {
		return png, "image/png", nil
	}
	return data, "application/octet-stream", nil
}

func readClipboardImageDarwinOsascript() ([]byte, string, error) {
	if _, err := exec.LookPath("osascript"); err != nil {
		return nil, "", err
	}
	classes := []struct {
		class string
		mime  string
	}{
		{"«class PNGf»", "image/png"},
		{"«class JPEG»", "image/jpeg"},
		{"«class TIFF»", "image/tiff"},
	}
	for _, c := range classes {
		data, err := readClipboardImageDarwinOsascriptClass(c.class)
		if err != nil || len(data) == 0 {
			continue
		}
		if c.mime == "image/png" {
			return data, c.mime, nil
		}
		if png, err := convertImageBytesToPNG(data); err == nil && len(png) > 0 {
			return png, "image/png", nil
		}
	}
	return nil, "", nil
}

func readClipboardImageDarwinOsascriptClass(class string) ([]byte, error) {
	f, err := os.CreateTemp("", "stell-clip-*")
	if err != nil {
		return nil, err
	}
	path := f.Name()
	_ = f.Close()
	defer os.Remove(path)
	script := `set imgData to the clipboard as ` + class + `
set outPath to "` + path + `"
set f to open for access POSIX file outPath with write permission
write imgData to f
close access f`
	if err := exec.Command("osascript", "-e", script).Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

func convertImageBytesToPNG(data []byte) ([]byte, error) {
	if _, err := exec.LookPath("sips"); err != nil {
		return nil, err
	}
	in, err := os.CreateTemp("", "stell-clip-in-*")
	if err != nil {
		return nil, err
	}
	inPath := in.Name()
	_ = in.Close()
	defer os.Remove(inPath)
	if err := os.WriteFile(inPath, data, 0o600); err != nil {
		return nil, err
	}
	out, err := os.CreateTemp("", "stell-clip-out-*.png")
	if err != nil {
		return nil, err
	}
	outPath := out.Name()
	_ = out.Close()
	defer os.Remove(outPath)
	if err := exec.Command("sips", "-s", "format", "png", inPath, "--out", outPath).Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(outPath)
}

func readClipboardImageLinux() ([]byte, string, error) {
	if _, err := exec.LookPath("wl-paste"); err == nil {
		data, err := exec.Command("wl-paste", "-t", "image/png").Output()
		if err == nil && len(data) > 0 {
			return data, "image/png", nil
		}
	}
	if _, err := exec.LookPath("xclip"); err == nil {
		data, err := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png", "-o").Output()
		if err == nil && len(data) > 0 {
			return data, "image/png", nil
		}
	}
	return nil, "", nil
}

func readClipboardImageWindows() ([]byte, string, error) {
	if _, err := exec.LookPath("powershell"); err != nil {
		return nil, "", nil
	}
	f, err := os.CreateTemp("", "stell-clip-*.png")
	if err != nil {
		return nil, "", err
	}
	path := f.Name()
	_ = f.Close()
	defer os.Remove(path)
	script := `Add-Type -AssemblyName System.Windows.Forms; if ([System.Windows.Forms.Clipboard]::ContainsImage()) { [System.Windows.Forms.Clipboard]::GetImage().Save('` + path + `') }`
	if err := exec.Command("powershell", "-NoProfile", "-Command", script).Run(); err != nil {
		return nil, "", err
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, "", nil
	}
	return data, "image/png", nil
}

func (m *Model) tryPasteClipboardImage() bool {
	data, _, err := readClipboardImageBytes()
	if err != nil {
		m.errLine = "Image paste failed: " + err.Error()
		return false
	}
	if len(data) == 0 {
		return false
	}
	path := filepath.Join(m.cfg.Workspace, ".stell-clipboard.png")
	if err := os.WriteFile(path, data, 0o600); err != nil {
		m.errLine = err.Error()
		return true
	}
	rel, _ := filepath.Rel(m.cfg.Workspace, path)
	if rel == "" || rel == "." {
		rel = ".stell-clipboard.png"
	}
	m.errLine = ""
	m.addAttachment(rel)
	return true
}
