module github.com/gitpod-io/xterm-go/_example

go 1.25.7

replace github.com/gitpod-io/xterm-go => ../

require (
	github.com/creack/pty v1.1.24
	github.com/gitpod-io/xterm-go v0.0.0-00010101000000-000000000000
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.4-0.20250319132907-e064f32e3674
)

require golang.org/x/net v0.52.0 // indirect
