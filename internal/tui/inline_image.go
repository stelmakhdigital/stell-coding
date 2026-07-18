package tui

import (
	"fmt"
	"os"
	"strings"
)

// renderInlineImage кодирует path как terminal image при showImages и подходящих caps.
func (m *Model) renderInlineImage(path, alt string, width int) string {
	if m.cfg == nil || !m.cfg.Settings.ImagesEnabled() {
		img := NewImage(path, alt)
		return strings.Join(img.Render(width), "\n")
	}
	caps := DetectCapabilities()
	if caps.Images == ImageNone {
		return ImageStub(width, 3, alt)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ImageStub(width, 3, alt)
	}
	mime := "image/png"
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".jpg"), strings.HasSuffix(lower, ".jpeg"):
		mime = "image/jpeg"
	case strings.HasSuffix(lower, ".gif"):
		mime = "image/gif"
	case strings.HasSuffix(lower, ".webp"):
		mime = "image/webp"
	}
	cells := m.cfg.Settings.ImageWidthCells
	if cells <= 0 || m.cfg.Settings.AutoResizeImagesEnabled() {
		cells = min(width, 40)
		if m.cellW > 0 && width > 0 {
			// При известных метриках ячейки — ~половина ширины терминала.
			cells = min(width/2, 60)
			if cells < 8 {
				cells = 8
			}
		}
	}
	enc := EncodeTerminalImage(caps.Images, mime, data, ImageRenderOptions{
		MaxWidthCells: cells,
		ImageID:       1 + len(path)%200,
	})
	if enc == "" {
		return ImageStub(width, 3, alt)
	}
	// Строка под изображение + подпись.
	return enc + "\n" + m.colors.muted().Render(fmt.Sprintf("[%s]", alt))
}
