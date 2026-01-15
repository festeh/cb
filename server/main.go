package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	imageDir  string
	configDir string

	// SSE clients
	sseClients   = make(map[chan string]bool)
	sseClientsMu sync.Mutex

	// Settings (loaded at startup)
	settings   *Settings
	settingsMu sync.RWMutex

	// Current flow execution state
	flowState   *FlowState
	flowStateMu sync.RWMutex

	// Current image selection (single source of truth)
	currentImage   string
	currentImageMu sync.RWMutex

	// Current tab selection
	currentTab   int
	currentTabMu sync.RWMutex
)

type FlowState struct {
	FlowID    string                       `json:"flow_id"`
	Running   bool                         `json:"running"`
	Step      int                          `json:"step"`
	Models    []string                     `json:"models"`
	Responses map[string]map[int]string    `json:"responses"` // model -> step -> text
}

type Step struct {
	Prompt string `json:"prompt"`
}

type Flow struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Steps  []Step   `json:"steps"`
	Models []string `json:"models"`
}

type Settings struct {
	OpenRouterAPIKey string `json:"openrouter_api_key"`
	Flows            []Flow `json:"flows"`
	DefaultFlowID    string `json:"default_flow_id"`
}

func main() {
	port := flag.Int("port", 8080, "port to listen on")
	dir := flag.String("dir", "", "directory to save images (default ~/Pictures/cb)")
	lan := flag.Bool("lan", false, "listen on all interfaces and show LAN IP")
	flag.Parse()

	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds)
	log.Println("Starting image server")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatalf("Failed to get home directory: %v", err)
	}

	if *dir != "" {
		imageDir = *dir
	} else {
		imageDir = filepath.Join(homeDir, "Pictures", "cb")
	}
	configDir = filepath.Join(homeDir, ".config", "cb")

	if err := os.MkdirAll(imageDir, 0755); err != nil {
		log.Fatalf("Failed to create image directory: %v", err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	settings, err = loadSettingsFromFile()
	if err != nil {
		log.Fatalf("Failed to load settings: %v", err)
	}

	currentImage = getMostRecentImage()
	log.Printf("Image directory: %s", imageDir)
	log.Printf("Config directory: %s", configDir)
	log.Printf("Current image: %s", currentImage)

	http.HandleFunc("/upload", handleUpload)
	http.HandleFunc("/images", handleImages)
	http.HandleFunc("/images/", handleImageFile)
	http.HandleFunc("/events", handleSSE)
	http.HandleFunc("/settings", handleSettings)
	http.HandleFunc("/flow", handleFlow)
	http.HandleFunc("/flow/state", handleFlowState)
	http.HandleFunc("/flow/trigger", handleFlowTrigger)
	http.HandleFunc("/flow/clear", handleFlowClear)
	http.HandleFunc("/flow/has-content", handleFlowHasContent)
	http.HandleFunc("/current-image", handleCurrentImage)
	http.HandleFunc("/tab", handleTab)
	http.HandleFunc("/scroll", handleScroll)

	var addr string
	if *lan {
		addr = fmt.Sprintf("0.0.0.0:%d", *port)
		if ip := getLocalIP(); ip != "" {
			log.Printf("LAN address: http://%s:%d", ip, *port)
		}
	} else {
		addr = fmt.Sprintf(":%d", *port)
	}
	log.Printf("Listening on %s", addr)
	if err := http.ListenAndServe(addr, nil); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}

func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() && ipnet.IP.To4() != nil {
			return ipnet.IP.String()
		}
	}
	return ""
}

func handleUpload(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	log.Printf("Receiving image from %s", r.RemoteAddr)
	start := time.Now()

	timestamp := time.Now().Format("2006-01-02_15-04-05")
	filename := fmt.Sprintf("img_%s.png", timestamp)
	fullPath := filepath.Join(imageDir, filename)

	file, err := os.Create(fullPath)
	if err != nil {
		log.Printf("ERROR: Failed to create file: %v", err)
		http.Error(w, "Failed to save file", http.StatusInternalServerError)
		return
	}
	defer file.Close()

	written, err := io.Copy(file, r.Body)
	if err != nil {
		log.Printf("ERROR: Failed to write file: %v", err)
		http.Error(w, "Failed to write file", http.StatusInternalServerError)
		return
	}

	elapsed := time.Since(start)
	log.Printf("Saved: %s (%d bytes, took %v)", fullPath, written, elapsed)

	// Update current image (source of truth)
	setCurrentImage(filename)

	// Clear flow state so next F1 triggers new flow
	flowStateMu.Lock()
	flowState = nil
	flowStateMu.Unlock()

	// Notify SSE clients
	broadcastSSE(filename)

	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Saved: %s", fullPath)
}

func handleImages(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(imageDir)
	if err != nil {
		log.Printf("ERROR: Failed to read image directory: %v", err)
		http.Error(w, "Failed to list images", http.StatusInternalServerError)
		return
	}

	images := []string{}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".png") {
			images = append(images, entry.Name())
		}
	}

	// Sort by name descending (newest first due to timestamp naming)
	sort.Sort(sort.Reverse(sort.StringSlice(images)))

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(images)
}

func handleImageFile(w http.ResponseWriter, r *http.Request) {
	filename := strings.TrimPrefix(r.URL.Path, "/images/")

	// Prevent path traversal
	if strings.Contains(filename, "..") || strings.Contains(filename, "/") {
		http.Error(w, "Invalid filename", http.StatusBadRequest)
		return
	}

	fullPath := filepath.Join(imageDir, filename)
	http.ServeFile(w, r, fullPath)
}

func handleSSE(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	clientChan := make(chan string, 10)

	sseClientsMu.Lock()
	sseClients[clientChan] = true
	sseClientsMu.Unlock()

	log.Printf("SSE client connected: %s (total: %d)", r.RemoteAddr, len(sseClients))

	// Send immediate connected event so browser knows connection is live
	fmt.Fprintf(w, "event: connected\ndata: ok\n\n")
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	defer func() {
		sseClientsMu.Lock()
		delete(sseClients, clientChan)
		sseClientsMu.Unlock()
		close(clientChan)
		log.Printf("SSE client disconnected: %s (total: %d)", r.RemoteAddr, len(sseClients))
	}()

	// Heartbeat ticker
	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case msg := <-clientChan:
			// Parse event|data format
			parts := strings.SplitN(msg, "|", 2)
			if len(parts) == 2 {
				fmt.Fprintf(w, "event: %s\ndata: %s\n\n", parts[0], parts[1])
			} else {
				fmt.Fprintf(w, "event: newimage\ndata: %s\n\n", msg)
			}
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-heartbeat.C:
			fmt.Fprintf(w, ": heartbeat\n\n")
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		case <-r.Context().Done():
			return
		}
	}
}

type SSEMessage struct {
	Event string
	Data  string
}

func broadcastSSE(filename string) {
	broadcastSSEEvent(SSEMessage{Event: "newimage", Data: filename})
}

func broadcastFlowUpdate() {
	flowStateMu.RLock()
	state := flowState
	if state == nil {
		flowStateMu.RUnlock()
		return
	}

	// Deep copy to avoid concurrent map access during JSON marshal
	stateCopy := &FlowState{
		FlowID:    state.FlowID,
		Running:   state.Running,
		Step:      state.Step,
		Models:    make([]string, len(state.Models)),
		Responses: make(map[string]map[int]string),
	}
	copy(stateCopy.Models, state.Models)
	for model, steps := range state.Responses {
		stateCopy.Responses[model] = make(map[int]string)
		for step, text := range steps {
			stateCopy.Responses[model][step] = text
		}
	}
	flowStateMu.RUnlock()

	data, err := json.Marshal(stateCopy)
	if err != nil {
		return
	}
	broadcastSSEEvent(SSEMessage{Event: "flowupdate", Data: string(data)})
}

func broadcastSSEEvent(msg SSEMessage) {
	sseClientsMu.Lock()
	defer sseClientsMu.Unlock()

	for clientChan := range sseClients {
		select {
		case clientChan <- msg.Event + "|" + msg.Data:
		default:
			log.Printf("SSE client buffer full, skipping")
		}
	}
}

func getSettingsPath() string {
	return filepath.Join(configDir, "settings.json")
}

func getMostRecentImage() string {
	entries, err := os.ReadDir(imageDir)
	if err != nil {
		return ""
	}

	var images []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".png") {
			images = append(images, entry.Name())
		}
	}

	if len(images) == 0 {
		return ""
	}

	// Sort descending (newest first due to timestamp naming)
	sort.Sort(sort.Reverse(sort.StringSlice(images)))
	return images[0]
}

func getCurrentImage() string {
	currentImageMu.RLock()
	defer currentImageMu.RUnlock()
	return currentImage
}

func setCurrentImage(img string) {
	currentImageMu.Lock()
	currentImage = img
	currentImageMu.Unlock()
}

func loadSettingsFromFile() (*Settings, error) {
	data, err := os.ReadFile(getSettingsPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Settings{}, nil
		}
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, err
	}
	return &s, nil
}

func getSettings() *Settings {
	settingsMu.RLock()
	defer settingsMu.RUnlock()
	return settings
}

func saveSettings(s *Settings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(getSettingsPath(), data, 0600); err != nil {
		return err
	}
	settingsMu.Lock()
	settings = s
	settingsMu.Unlock()
	return nil
}

func handleSettings(w http.ResponseWriter, r *http.Request) {
	log.Printf("Settings request: %s", r.Method)
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(getSettings())
		log.Printf("Settings GET: returned %d flows", len(getSettings().Flows))

	case http.MethodPost:
		var s Settings
		if err := json.NewDecoder(r.Body).Decode(&s); err != nil {
			log.Printf("Settings POST: invalid JSON: %v", err)
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		log.Printf("Settings POST: received %d flows, default=%s", len(s.Flows), s.DefaultFlowID)
		if err := saveSettings(&s); err != nil {
			log.Printf("ERROR: Failed to save settings: %v", err)
			http.Error(w, "Failed to save settings", http.StatusInternalServerError)
			return
		}
		log.Printf("Settings saved successfully")
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleCurrentImage(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"image": getCurrentImage()})

	case http.MethodPost:
		var req struct {
			Image string `json:"image"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		setCurrentImage(req.Image)
		log.Printf("Current image updated: %s", req.Image)
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleTab(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		currentTabMu.RLock()
		tab := currentTab
		currentTabMu.RUnlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]int{"tab": tab})

	case http.MethodPost:
		var req struct {
			Tab int `json:"tab"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON", http.StatusBadRequest)
			return
		}
		currentTabMu.Lock()
		currentTab = req.Tab
		currentTabMu.Unlock()
		log.Printf("Tab changed: %d", req.Tab)

		// Broadcast tab change
		broadcastSSEEvent(SSEMessage{Event: "tabchange", Data: strconv.Itoa(req.Tab)})
		w.WriteHeader(http.StatusOK)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func handleScroll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dir := r.URL.Query().Get("dir")
	if dir != "up" && dir != "down" {
		http.Error(w, "Invalid direction: %s", http.StatusBadRequest)
		return
	}

	log.Printf("Scroll: %s (clients: %d)", dir, len(sseClients))
	broadcastSSEEvent(SSEMessage{Event: "scroll", Data: dir})
	w.WriteHeader(http.StatusOK)
}

func handleFlowClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flowStateMu.Lock()
	flowState = nil
	flowStateMu.Unlock()

	log.Printf("Flow state cleared")
	w.WriteHeader(http.StatusOK)
}

func handleFlowHasContent(w http.ResponseWriter, r *http.Request) {
	flowStateMu.RLock()
	state := flowState
	flowStateMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"has_content": flowHasContent(state)})
}

func flowHasContent(state *FlowState) bool {
	if state == nil || state.Responses == nil {
		return false
	}
	for _, steps := range state.Responses {
		for _, text := range steps {
			if len(text) > 0 {
				return true
			}
		}
	}
	return false
}

func handleFlowTrigger(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	img := getCurrentImage()
	if img == "" {
		http.Error(w, "No image selected", http.StatusBadRequest)
		return
	}

	s := getSettings()
	if s.DefaultFlowID == "" {
		http.Error(w, "No default flow configured", http.StatusBadRequest)
		return
	}

	// Create internal request to handleFlow
	req := FlowRequest{
		Image:  img,
		FlowID: s.DefaultFlowID,
	}
	reqBody, _ := json.Marshal(req)

	// Forward to handleFlow
	newReq, _ := http.NewRequest(http.MethodPost, "/flow", bytes.NewReader(reqBody))
	newReq.Header.Set("Content-Type", "application/json")
	handleFlow(w, newReq)
}

func handleFlowState(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	model := r.URL.Query().Get("model")
	tabStr := r.URL.Query().Get("tab")

	flowStateMu.RLock()
	state := flowState
	flowStateMu.RUnlock()

	w.Header().Set("Content-Type", "application/json")
	if state == nil {
		json.NewEncoder(w).Encode(map[string]interface{}{"running": false})
		return
	}

	// Convert tab index to model name
	if tabStr != "" && model == "" {
		tab, err := strconv.Atoi(tabStr)
		if err == nil && tab >= 0 && tab < len(state.Models) {
			model = state.Models[tab]
		}
	}

	if model != "" {
		// Return only the specified model's responses
		filtered := map[string]interface{}{
			"flow_id": state.FlowID,
			"running": state.Running,
			"step":    state.Step,
			"model":   model,
			"content": state.Responses[model],
		}
		json.NewEncoder(w).Encode(filtered)
		return
	}

	json.NewEncoder(w).Encode(state)
}

type FlowRequest struct {
	Image  string `json:"image"`
	FlowID string `json:"flow_id"`
}

type StreamChunk struct {
	Model   string `json:"model,omitempty"`
	Step    int    `json:"step"`
	Content string `json:"content,omitempty"`
	Done    bool   `json:"done,omitempty"`
	Error   string `json:"error,omitempty"`
}

func handleFlow(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req FlowRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Get settings
	s := getSettings()
	if s.OpenRouterAPIKey == "" {
		http.Error(w, "OpenRouter API key not configured", http.StatusBadRequest)
		return
	}

	// Find flow
	var flow *Flow
	for i := range s.Flows {
		if s.Flows[i].ID == req.FlowID {
			flow = &s.Flows[i]
			break
		}
	}
	if flow == nil {
		http.Error(w, "Flow not found", http.StatusBadRequest)
		return
	}
	if len(flow.Steps) == 0 {
		http.Error(w, "Flow has no steps", http.StatusBadRequest)
		return
	}
	if len(flow.Models) == 0 {
		http.Error(w, "Flow has no models", http.StatusBadRequest)
		return
	}

	// Read and encode image
	if strings.Contains(req.Image, "..") || strings.Contains(req.Image, "/") {
		http.Error(w, "Invalid image filename", http.StatusBadRequest)
		return
	}
	imagePath := filepath.Join(imageDir, req.Image)
	imageData, err := os.ReadFile(imagePath)
	if err != nil {
		log.Printf("ERROR: Failed to read image: %v", err)
		http.Error(w, "Failed to read image", http.StatusInternalServerError)
		return
	}
	imageBase64 := base64.StdEncoding.EncodeToString(imageData)

	// Setup SSE response
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	// Channel for streaming chunks
	chunks := make(chan StreamChunk, 100)

	// Track results per model for each step
	modelResults := make(map[string]string)

	// Initialize flow state
	flowStateMu.Lock()
	flowState = &FlowState{
		FlowID:    req.FlowID,
		Running:   true,
		Step:      0,
		Models:    flow.Models,
		Responses: make(map[string]map[int]string),
	}
	for _, model := range flow.Models {
		flowState.Responses[model] = make(map[int]string)
	}
	flowStateMu.Unlock()
	broadcastFlowUpdate()

	// Use request context for cancellation
	ctx := r.Context()

	// Run flow steps
	go func() {
		defer close(chunks)
		defer func() {
			flowStateMu.Lock()
			if flowState != nil {
				flowState.Running = false
			}
			flowStateMu.Unlock()
			broadcastFlowUpdate()
		}()

		for stepIdx, step := range flow.Steps {
			// Update current step
			flowStateMu.Lock()
			if flowState != nil {
				flowState.Step = stepIdx
			}
			flowStateMu.Unlock()

			// Check if cancelled
			select {
			case <-ctx.Done():
				return
			default:
			}

			var wg sync.WaitGroup
			var mu sync.Mutex

			for _, model := range flow.Models {
				wg.Add(1)
				go func(model string, stepIdx int, prompt string) {
					defer wg.Done()
					runModelStep(ctx, s.OpenRouterAPIKey, model, stepIdx, prompt, imageBase64, modelResults, &mu, chunks)
				}(model, stepIdx, step.Prompt)
			}

			wg.Wait()

			// Check if cancelled before sending done marker
			select {
			case <-ctx.Done():
				return
			default:
				chunks <- StreamChunk{Step: stepIdx, Done: true}
			}
		}
	}()

	// Stream chunks to client
	for chunk := range chunks {
		select {
		case <-ctx.Done():
			return
		default:
			data, _ := json.Marshal(chunk)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}
	}

	fmt.Fprintf(w, "data: [DONE]\n\n")
	flusher.Flush()
}

func redactImages(messages []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, len(messages))
	for i, msg := range messages {
		result[i] = redactMessage(msg)
	}
	return result
}

func redactMessage(msg map[string]interface{}) map[string]interface{} {
	newMsg := make(map[string]interface{})
	for k, v := range msg {
		if k != "content" {
			newMsg[k] = v
			continue
		}
		content, ok := v.([]map[string]interface{})
		if !ok {
			newMsg[k] = v
			continue
		}
		newMsg[k] = redactContent(content)
	}
	return newMsg
}

func redactContent(content []map[string]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, len(content))
	for i, item := range content {
		newItem := make(map[string]interface{})
		for k, v := range item {
			if k == "image_url" {
				newItem[k] = "[REDACTED]"
			} else {
				newItem[k] = v
			}
		}
		result[i] = newItem
	}
	return result
}

func runModelStep(ctx context.Context, apiKey, model string, stepIdx int, prompt, imageBase64 string, modelResults map[string]string, mu *sync.Mutex, chunks chan<- StreamChunk) {
	// Build messages
	var messages []map[string]interface{}

	if stepIdx == 0 {
		// First step: include image
		messages = []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": prompt},
					{"type": "image_url", "image_url": map[string]string{"url": "data:image/png;base64," + imageBase64}},
				},
			},
		}
	} else {
		// Subsequent steps: include previous result
		mu.Lock()
		prevResult := modelResults[model]
		mu.Unlock()

		messages = []map[string]interface{}{
			{
				"role": "user",
				"content": []map[string]interface{}{
					{"type": "text", "text": prompt},
					{"type": "image_url", "image_url": map[string]string{"url": "data:image/png;base64," + imageBase64}},
				},
			},
			{
				"role":    "assistant",
				"content": prevResult,
			},
			{
				"role":    "user",
				"content": prompt,
			},
		}
	}

	openRouterReq := map[string]interface{}{
		"model":    model,
		"stream":   true,
		"messages": messages,
	}

	reqBody, err := json.Marshal(openRouterReq)
	if err != nil {
		chunks <- StreamChunk{Model: model, Step: stepIdx, Error: err.Error()}
		return
	}

	// Log request with redacted image data
	logReq := map[string]interface{}{
		"model":    model,
		"stream":   true,
		"messages": redactImages(messages),
	}
	if logJSON, err := json.Marshal(logReq); err == nil {
		log.Printf("AI request: %s", logJSON)
	}

	client := &http.Client{}
	httpReq, err := http.NewRequestWithContext(ctx, "POST", "https://openrouter.ai/api/v1/chat/completions", bytes.NewReader(reqBody))
	if err != nil {
		chunks <- StreamChunk{Model: model, Step: stepIdx, Error: err.Error()}
		return
	}
	httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(httpReq)
	if err != nil {
		log.Printf("AI error [%s]: %v", model, err)
		chunks <- StreamChunk{Model: model, Step: stepIdx, Error: err.Error()}
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Printf("AI error [%s]: status %d: %s", model, resp.StatusCode, body)
		chunks <- StreamChunk{Model: model, Step: stepIdx, Error: string(body)}
		return
	}

	var fullResponse strings.Builder
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if err := json.Unmarshal([]byte(data), &chunk); err == nil {
				if len(chunk.Choices) > 0 && chunk.Choices[0].Delta.Content != "" {
					content := chunk.Choices[0].Delta.Content
					fullResponse.WriteString(content)
					chunks <- StreamChunk{Model: model, Step: stepIdx, Content: content}

					// Update flow state
					flowStateMu.Lock()
					if flowState != nil && flowState.Responses[model] != nil {
						flowState.Responses[model][stepIdx] = fullResponse.String()
					}
					flowStateMu.Unlock()
					broadcastFlowUpdate()
				}
			}
		}
	}

	// Log response
	logResp := map[string]interface{}{
		"model":    model,
		"step":     stepIdx,
		"response": fullResponse.String(),
	}
	if logJSON, err := json.Marshal(logResp); err == nil {
		log.Printf("AI response: %s", logJSON)
	}

	// Store result for next step
	mu.Lock()
	modelResults[model] = fullResponse.String()
	mu.Unlock()
}
