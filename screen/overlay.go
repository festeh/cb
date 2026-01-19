package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/diamondburned/gotk4-layer-shell/pkg/gtklayershell"
	"github.com/diamondburned/gotk4/pkg/gdk/v3"
	"github.com/diamondburned/gotk4/pkg/glib/v2"
	"github.com/diamondburned/gotk4/pkg/gtk/v3"
	"github.com/spf13/cobra"
)

var (
	tabIndex    int
	position    string
	stepIndex   int
	showOverlay bool
)

var overlayCmd = &cobra.Command{
	Use:   "overlay",
	Short: "Display screen overlay with flow content",
	Run:   runOverlay,
}

func init() {
	overlayCmd.Flags().IntVarP(&tabIndex, "tab", "t", 0, "Tab index (0-based)")
	overlayCmd.Flags().StringVarP(&position, "pos", "p", "", "Position (left, center, right)")
	overlayCmd.Flags().IntVar(&stepIndex, "step", -1, "Step index (0-based, -1 for all)")
	overlayCmd.Flags().BoolVar(&showOverlay, "show-overlay", true, "Show GTK overlay (false to only send commands to frontend)")
}

type ScreenState struct {
	Tab      int    `json:"tab"`
	Position string `json:"position"`
	Server   string `json:"server"`
}

func (s ScreenState) IsRunningWith(tab int, pos string) bool {
	return isOverlayRunning() && s.Tab == tab && s.Position == pos
}

type FlowStateResponse struct {
	Running bool              `json:"running"`
	Step    int               `json:"step"`
	Model   string            `json:"model"`
	Content map[string]string `json:"content"`
}

func getStateFilePath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "cb", "screen-state.json")
}

func getPidFilePath() string {
	return "/tmp/cb-screen-overlay.pid"
}

func loadScreenState() ScreenState {
	state := ScreenState{Tab: 0, Position: "right", Server: "http://localhost:8080"}
	data, err := os.ReadFile(getStateFilePath())
	if err != nil {
		return state
	}
	json.Unmarshal(data, &state)
	return state
}

func saveScreenState(state ScreenState) {
	dir := filepath.Dir(getStateFilePath())
	os.MkdirAll(dir, 0755)
	data, _ := json.Marshal(state)
	os.WriteFile(getStateFilePath(), data, 0644)
}

func isOverlayRunning() bool {
	data, err := os.ReadFile(getPidFilePath())
	if err != nil {
		return false
	}
	pid := strings.TrimSpace(string(data))
	// Check if process exists
	err = exec.Command("kill", "-0", pid).Run()
	return err == nil
}

func killOverlay() {
	data, err := os.ReadFile(getPidFilePath())
	if err != nil {
		return
	}
	pid := strings.TrimSpace(string(data))
	exec.Command("kill", pid).Run()
	os.Remove(getPidFilePath())
}

func writePidFile() {
	os.WriteFile(getPidFilePath(), []byte(fmt.Sprintf("%d", os.Getpid())), 0644)
}

func notifyTabChange(server string, tab int) {
	body, _ := json.Marshal(map[string]int{"tab": tab})
	resp, err := http.Post(server+"/tab", "application/json", bytes.NewReader(body))
	if err != nil {
		log.Printf("Failed to notify tab change: %v", err)
		return
	}
	resp.Body.Close()
}

func triggerFlowIfNeeded(server string) {
	resp, err := http.Get(server + "/flow/has-content")
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var data struct {
		HasContent bool `json:"has_content"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return
	}

	if !data.HasContent {
		http.Post(server+"/flow/trigger", "", nil)
	}
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

func subscribeSSE(updateFunc func(*FlowStateResponse)) {
	for {
		log.Printf("Connecting to SSE...")
		resp, err := http.Get(serverURL + "/events")
		if err != nil {
			log.Printf("SSE connection failed: %v, retrying in 5s", err)
			time.Sleep(5 * time.Second)
			continue
		}

		log.Printf("SSE connected")
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()
			if !strings.HasPrefix(line, "data: ") {
				continue
			}
			data := strings.TrimPrefix(line, "data: ")
			parts := strings.SplitN(data, "|", 2)
			if len(parts) != 2 {
				continue
			}
			event, payload := parts[0], parts[1]
			if event != "flowupdate" {
				continue
			}

			var state FlowStateResponse
			if err := json.Unmarshal([]byte(payload), &state); err != nil {
				log.Printf("Failed to parse flowupdate: %v", err)
				continue
			}
			log.Printf("Got flow update, running=%v, content keys=%d", state.Running, len(state.Content))
			updateFunc(&state)
		}

		resp.Body.Close()
		log.Printf("SSE disconnected, reconnecting in 1s")
		time.Sleep(1 * time.Second)
	}
}

func runOverlay(cmd *cobra.Command, args []string) {
	savedState := loadScreenState()

	// Apply defaults from saved state if flags not provided
	if !cmd.Flags().Changed("tab") {
		tabIndex = savedState.Tab
	}
	if !cmd.Flags().Changed("pos") {
		position = savedState.Position
	}
	if !cmd.Flags().Changed("server") {
		serverURL = savedState.Server
	}

	log.Printf("Tab: %d, Position: %s, Server: %s, ShowOverlay: %v", tabIndex, position, serverURL, showOverlay)

	// Toggle: if running with same params, kill and exit
	if showOverlay && savedState.IsRunningWith(tabIndex, position) {
		log.Printf("Overlay running with same params, toggling off")
		killOverlay()
		os.Exit(0)
	}

	// Save new state
	saveScreenState(ScreenState{Tab: tabIndex, Position: position, Server: serverURL})

	// Notify server of tab change
	notifyTabChange(serverURL, tabIndex)

	// Trigger flow if needed
	triggerFlowIfNeeded(serverURL)

	if !showOverlay {
		log.Printf("Commands sent to frontend, not showing overlay")
		return
	}

	// Kill any existing overlay
	killOverlay()

	// Write PID file
	writePidFile()

	log.Printf("Starting screen overlay")

	app := gtk.NewApplication("com.github.festeh.cb.screen", 0)

	app.ConnectActivate(func() {
		win := gtk.NewWindow(gtk.WindowToplevel)
		win.SetTitle("Screen Overlay")
		win.SetDefaultSize(3200, 3200)

		gtklayershell.InitForWindow(win)
		gtklayershell.SetLayer(win, gtklayershell.LayerShellLayerOverlay)
		gtklayershell.SetKeyboardMode(win, gtklayershell.LayerShellKeyboardModeNone)
		gtklayershell.SetExclusiveZone(win, -1)

		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeTop, true)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeRight, true)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeBottom, true)
		gtklayershell.SetAnchor(win, gtklayershell.LayerShellEdgeLeft, true)
		gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeTop, 20)
		gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeBottom, 20)

		log.Printf("Position: %s", position)
		switch position {
		case "left":
			gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeLeft, 20)
			gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeRight, 800)
		case "center":
			gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeLeft, 400)
			gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeRight, 400)
		default:
			gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeLeft, 800)
			gtklayershell.SetMargin(win, gtklayershell.LayerShellEdgeRight, 20)
		}

		label := gtk.NewLabel("Connecting...")
		label.SetLineWrap(true)
		label.SetMaxWidthChars(80)
		label.SetVAlign(gtk.AlignStart)
		label.SetHAlign(gtk.AlignFill)

		scroll := gtk.NewScrolledWindow(nil, nil)
		scroll.SetPolicy(gtk.PolicyAutomatic, gtk.PolicyAutomatic)
		scroll.Add(label)

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

		box := gtk.NewBox(gtk.OrientationVertical, 0)
		box.SetMarginTop(20)
		box.SetMarginBottom(20)
		box.SetMarginStart(20)
		box.SetMarginEnd(20)
		box.PackStart(scroll, true, true, 0)

		win.Add(box)

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

		win.ConnectMap(func() {
			SetClickThrough(win.Native())
		})

		state, err := fetchFlowState()
		if err != nil {
			label.SetText("Not ready")
		} else {
			label.SetText(getDisplayText(state))
		}

		go subscribeSSE(func(state *FlowStateResponse) {
			glib.IdleAdd(func() {
				label.SetText(getDisplayText(state))
			})
		})

		app.AddWindow(win)
		win.ShowAll()
	})

	os.Exit(app.Run(nil))
}
