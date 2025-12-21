# CB Server

Go HTTP server for the CB screenshot analysis tool.

## Usage

```bash
go run main.go [-port 8080] [-dir ~/Pictures/cb] [-frontend ./frontend/dist]
```

## Configuration

Settings stored in `~/.config/cb/settings.json`:

```json
{
  "openrouter_api_key": "sk-or-...",
  "flows": [...],
  "default_flow_id": "uuid"
}
```

## API Endpoints

### Images

- `POST /upload` - Upload image (multipart form, field: `image`)
- `GET /images` - List images (JSON array of filenames)
- `GET /images/{filename}` - Serve image file

### Settings

- `GET /settings` - Get current settings
- `POST /settings` - Save settings

### Flow Execution

- `POST /flow` - Execute flow (SSE stream)
  ```json
  {"image": "filename.png", "flow_id": "uuid"}
  ```

- `GET /flow/state` - Get current flow execution state
  ```json
  {
    "flow_id": "uuid",
    "running": true,
    "step": 0,
    "responses": {
      "model/name": {"0": "response text..."}
    }
  }
  ```

- `GET /flow/state?model=anthropic/claude-sonnet-4` - Get single model's state
- `GET /flow/state?tab=0` - Get state by tab index (0-based)
  ```json
  {
    "flow_id": "uuid",
    "running": true,
    "step": 0,
    "model": "anthropic/claude-sonnet-4",
    "content": {"0": "response text..."}
  }
  ```

### Server-Sent Events

- `GET /events` - SSE connection for real-time updates
  - Event: `newimage` - New image uploaded

## Flow Execution

Flows support multi-step, multi-model execution:

1. Each step runs all models in parallel
2. Results are streamed back via SSE
3. Previous step results are passed to subsequent steps
4. Supports cancellation via request context
