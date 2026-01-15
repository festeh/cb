# Run all services
all: server frontend

# Run Go server
server *ARGS:
    go run ./server {{ARGS}}

# Run frontend dev server
frontend *ARGS:
    cd frontend && npm run dev -- {{ARGS}}

# Build frontend
build-frontend:
    cd frontend && npm run build

# Build server binary
build-server:
    cd server && go build -o server main.go

# Build screen binary (uses ~/.cache/go-build)
build-screen:
    cd screen && go build -v -o screen .

# Build all binaries
build: build-server build-screen build-frontend

# Launch screen overlay
screen *ARGS:
    bun screen/launch.ts {{ARGS}}

# Install dependencies
install:
    cd frontend && npm install
    cd screen && bun install
