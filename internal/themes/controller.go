package themes

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Controller применяет настройки темы, auto light/dark sync и опциональный hot-reload.
type Controller struct {
	opts           ResolveOpts
	setting        string // raw settings.theme (may be "light/dark")
	terminalTheme  string // "light" or "dark"
	autoSync       bool
	current        Theme
	onChange       func(Theme)
	mu             sync.Mutex
	stopWatch      chan struct{}
	watchOnce      sync.Once
}

func NewController(opts ResolveOpts, onChange func(Theme)) *Controller {
	return &Controller{
		opts:          opts,
		terminalTheme: DetectDefaultName(),
		onChange:      onChange,
	}
}

// ApplyFromSettings резолвит setting и применяет конкретную тему.
// Форма auto "light/dark" включает sync с определением темы терминала.
func (c *Controller) ApplyFromSettings(setting string) Theme {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.setting = strings.TrimSpace(setting)
	_, _, auto := ParseAutoThemeSetting(c.setting)
	c.autoSync = auto
	name := ResolveThemeSetting(c.setting, c.terminalTheme)
	if name == "" {
		name = c.terminalTheme
	}
	return c.applyLocked(name)
}

// SetTerminalTheme обновляет определённую тему терминала; переприменяет при включённом auto sync.
func (c *Controller) SetTerminalTheme(term string) Theme {
	c.mu.Lock()
	defer c.mu.Unlock()
	if term != "light" && term != "dark" {
		term = "dark"
	}
	c.terminalTheme = term
	if !c.autoSync {
		return c.current
	}
	name := ResolveThemeSetting(c.setting, c.terminalTheme)
	return c.applyLocked(name)
}

// SetThemeName применяет фиксированную тему и отключает auto sync.
func (c *Controller) SetThemeName(name string) Theme {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.autoSync = false
	c.setting = name
	return c.applyLocked(name)
}

// Current возвращает активную тему.
func (c *Controller) Current() Theme {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.current
}

// StartHotReload следит за GlobalDir/themes на изменения файла активной темы.
func (c *Controller) StartHotReload() {
	c.watchOnce.Do(func() {
		c.stopWatch = make(chan struct{})
		go c.watchLoop()
	})
}

// StopHotReload останавливает watcher.
func (c *Controller) StopHotReload() {
	c.mu.Lock()
	ch := c.stopWatch
	c.mu.Unlock()
	if ch != nil {
		select {
		case <-ch:
		default:
			close(ch)
		}
	}
}

func (c *Controller) applyLocked(name string) Theme {
	t := FindByName(c.opts, name)
	if t == nil {
		def := DefaultTheme()
		t = &def
	}
	c.current = *t
	if c.onChange != nil {
		c.onChange(c.current)
	}
	return c.current
}

func (c *Controller) watchLoop() {
	dir := ""
	c.mu.Lock()
	if c.opts.GlobalDir != "" {
		dir = filepath.Join(c.opts.GlobalDir, "themes")
	}
	stop := c.stopWatch
	c.mu.Unlock()
	if dir == "" || stop == nil {
		return
	}
	_ = os.MkdirAll(dir, 0o755)

	var lastMod time.Time
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			c.mu.Lock()
			cur := c.current
			c.mu.Unlock()
			if cur.Path() == "" {
				continue
			}
			// Следим только за темами в GlobalDir/themes
			if dir != "" && !pathUnder(cur.Path(), dir) {
				continue
			}
			info, err := os.Stat(cur.Path())
			if err != nil {
				continue
			}
			mod := info.ModTime()
			if lastMod.IsZero() {
				lastMod = mod
				continue
			}
			if !mod.After(lastMod) {
				continue
			}
			lastMod = mod
			time.Sleep(100 * time.Millisecond) // debounce
			reloaded, err := Load(cur.Path())
			if err != nil || reloaded.Validate() != nil {
				continue
			}
			c.mu.Lock()
			if c.current.Name != reloaded.Name && c.current.Path() != reloaded.Path() {
				c.mu.Unlock()
				continue
			}
			c.current = *reloaded
			cb := c.onChange
			c.mu.Unlock()
			if cb != nil {
				cb(*reloaded)
			}
		}
	}
}

func pathUnder(path, dir string) bool {
	absP, err1 := filepath.Abs(path)
	absD, err2 := filepath.Abs(dir)
	if err1 != nil || err2 != nil {
		return false
	}
	rel, err := filepath.Rel(absD, absP)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
