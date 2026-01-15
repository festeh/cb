package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/diamondburned/gotk4-layer-shell/pkg/gtklayershell"
	"github.com/diamondburned/gotk4/pkg/gdk/v3"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
)

type ScreenState struct {
	Position string `json:"position"` // "left", "center", "right"
}

func getStateFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cb", "screen-state.json")
}

func loadScreenState() ScreenState {
	state := ScreenState{Position: "right"} // default
	data, err := os.ReadFile(getStateFilePath())
	if err != nil {
		return state
	}
	json.Unmarshal(data, &state)
	return state
}

var (
	serverURL string
	tabIndex  int
	stepIndex int
)

type FlowStateResponse struct {
	Running bool              `json:"running"`
	Step    int               `json:"step"`
	Model   string            `json:"model"`
	Content map[string]string `json:"content"`
}

func fetchFlowState() (*FlowStateResponse, error) {
	url := fmt.Sprintf("%s/flow/state?tab=%d", serverURL, tabIndex)
	resp, err := http.Get(url)
	if err != nil {
		log.Printf("ERROR: fetch failed: %v", err)
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("ERROR: read body failed: %v", err)
		return nil, err
	}

	log.Printf("Server response: %s", body)

	var state FlowStateResponse
	if err := json.Unmarshal(body, &state); err != nil {
		log.Printf("ERROR: JSON parse failed: %v", err)
		return nil, err
	}
	return &state, nil
}

func getDisplayText(state *FlowStateResponse) string {
	if state == nil || !state.Running && len(state.Content) == 0 {
		return "No active flow"
	}

	// Single step requested
	if stepIndex >= 0 {
		text, ok := state.Content[fmt.Sprintf("%d", stepIndex)]
		if !ok || text == "" {
			if state.Running {
				return "Running..."
			}
			return "No content for step"
		}
		return text
	}

	// All steps
	var parts []string
	for step := 0; ; step++ {
		text, ok := state.Content[fmt.Sprintf("%d", step)]
		if !ok {
			break
		}
		if text != "" {
			parts = append(parts, text)
		}
	}

	if len(parts) == 0 {
		if state.Running {
			return "Running..."
		}
		return "No content"
	}

	return strings.Join(parts, "\n\n")
}

func main() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)

	flag.StringVar(&serverURL, "server", "http://localhost:8080", "Server URL")
	flag.IntVar(&tabIndex, "tab", 0, "Tab index (0-based)")
	flag.IntVar(&stepIndex, "step", -1, "Step index (0-based, -1 for all)")
	flag.Parse()

	log.Printf("Starting screen overlay")
	log.Printf("Server: %s", serverURL)
	log.Printf("Tab: %d", tabIndex)
	log.Printf("Step: %d", stepIndex)

	app := gtk.NewApplication("com.github.festeh.cb.screen", 0)

	app.ConnectActivate(func() {
		// Create the window
		win := gtk.NewWindow(gtk.WindowToplevel)
		win.SetTitle("Screen Overlay")
		win.SetDefaultSize(3200, 3200)

		// Initialize layer shell
		gtklayershell.InitForWindow(win)
		gtklayershell.SetLayer(win, gtklayershell.LayerShellLayerOverlay)
		gtklayershell.SetKeyboardMode(win, gtklayershell.LayerShellKeyboardModeNone)
		gtklayershell.SetExclusiveZone(win, -1) // IGNORE exclusivity

		// Anchor to all edges
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeTop, true)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeRight, true)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeBottom, true)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeLeft, true)
		gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeTop, 20)
		gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeBottom, 20)

		// Position based on saved state
		screenState := loadScreenState()
		log.Printf("Position: %s", screenState.Position)
		switch screenState.Position {
		case "left":
			gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeLeft, 20)
			gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeRight, 800)
		case "center":
			gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeLeft, 400)
			gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeRight, 400)
		default: // right
			gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeLeft, 800)
			gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeRight, 20)
		}

		// Create label
		label := gtk.NewLabel("Connecting...")
		label.SetLineWrap(true)
		label.SetMaxWidthChars(80)
		label.SetVAlign(gtk.AlignStart)
		label.SetHAlign(gtk.AlignFill)

		// Create a scrolled window
		scroll := gtk.NewScrolledWindow(nil, nil)
		scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
		scroll.Add(label)

		// Set up signal handlers for scrolling (SIGUSR1=up, SIGUSR2=down)
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGUSR1, syscall.SIGUSR2)
		go func() {
			for sig := range sigChan {
				glib.IdleAdd(func() {
					adj := scroll.VAdjustment()
					step := 200.0
					if sig == syscall.SIGUSR1 {
						adj.SetValue(adj.Value() - step)
					} else {
						adj.SetValue(adj.Value() + step)
					}
				})
			}
		}()

		// Create a box to hold content with padding
		box := gtk.NewBox(gtk.OrientationVertical, 0)
		box.SetMarginTop(20)
		box.SetMarginBottom(20)
		box.SetMarginStart(20)
		box.SetMarginEnd(20)
		box.PackStart(scroll, true, true, 0)

		win.Add(box)

		// Apply CSS
		css := gtk.NewCSSProvider()
		css.LoadFromData(`
			window, scrolledwindow, viewport, box {
				background-color: transparent;
			}
			label {
				background-color: rgba(0, 0, 0, 0.75);
				border-radius: 8px;
				padding: 12px;
				color: white;
				font-size: 14px;
				font-family: monospace;
			}
		`)
		screen := gdk.ScreenGetDefault()
		gtk.StyleContextAddProviderForScreen(
			screen,
			css,
			gtk.STYLE_PROVIDER_PRIORITY_APPLICATION,
		)

		// Set up click-through after window is mapped
		win.ConnectMap(func() {
			SetClickThrough(win.Native())
		})

		// Fetch state once and display
		state, err := fetchFlowState()
		if err != nil {
			label.SetText(fmt.Sprintf("Error: %v", err))
		} else {
			label.SetText(getDisplayText(state))
		}

		app.AddWindow(win)
		win.ShowAll()
	})

	os.Exit(app.Run(nil))
}
