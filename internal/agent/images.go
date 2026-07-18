package agent

import (
	"encoding/base64"
	"fmt"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"github.com/stelmakhdigital/stell-ai"
	"github.com/stelmakhdigital/stell-coding/internal/config"
)

var imageExts = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".bmp":  "image/bmp",
}

func IsImagePath(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	_, ok := imageExts[ext]
	return ok
}

func SplitAttachments(paths []string) (images, files []string) {
	for _, p := range paths {
		if IsImagePath(p) {
			images = append(images, p)
		} else {
			files = append(files, p)
		}
	}
	return images, files
}

func LoadImageFromPath(workspace, rel string) (ai.ImageContent, error) {
	abs := rel
	if !filepath.IsAbs(rel) {
		abs = filepath.Join(workspace, rel)
	}
	abs = filepath.Clean(abs)
	root := filepath.Clean(workspace)
	if !strings.HasPrefix(abs, root+string(os.PathSeparator)) && abs != root {
		return ai.ImageContent{}, fmt.Errorf("image outside workspace: %s", rel)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return ai.ImageContent{}, err
	}
	mimeType := imageExts[strings.ToLower(filepath.Ext(rel))]
	if mimeType == "" {
		mimeType = mime.TypeByExtension(filepath.Ext(rel))
	}
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}
	return ai.ImageContent{
		Type:     "image",
		Data:     base64.StdEncoding.EncodeToString(data),
		MimeType: mimeType,
	}, nil
}

func LoadImages(workspace string, paths []string) ([]ai.ImageContent, error) {
	out := make([]ai.ImageContent, 0, len(paths))
	for _, p := range paths {
		img, err := LoadImageFromPath(workspace, p)
		if err != nil {
			return nil, err
		}
		out = append(out, img)
	}
	return out, nil
}

// DefaultImagePrompt используется, когда пользователь отправляет изображения без текста.
const DefaultImagePrompt = "Describe the attached image(s)."

func PromptWithImages(text string, images []ai.ImageContent) string {
	if strings.TrimSpace(text) == "" && len(images) > 0 {
		return DefaultImagePrompt
	}
	return text
}

func ModelSupportsImages(mc config.ModelConfig) bool {
	return mc.SupportsImage()
}

func BuildUserMessage(text string, images []ai.ImageContent, mc config.ModelConfig) ai.Message {
	msg := ai.Message{Role: ai.RoleUser, Content: text}
	if ModelSupportsImages(mc) && len(images) > 0 {
		msg.Images = images
		return msg
	}
	if len(images) > 0 {
		msg.Content = expandImages(text, images)
	}
	return msg
}

func expandImages(message string, images []ai.ImageContent) string {
	if len(images) == 0 {
		return message
	}
	var b strings.Builder
	if message != "" {
		b.WriteString(message)
		b.WriteString("\n\n")
	}
	for i, img := range images {
		mime := img.MimeType
		if mime == "" {
			mime = "application/octet-stream"
		}
		size := len(img.Data)
		fmt.Fprintf(&b, "[image %d: %s, %d bytes base64]\n", i+1, mime, size)
	}
	return strings.TrimSpace(b.String())
}

func IsMultimodalUnsupportedError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "multimodal") ||
		strings.Contains(msg, "does not support multimodal") ||
		strings.Contains(msg, "image input") ||
		strings.Contains(msg, "image_url")
}
