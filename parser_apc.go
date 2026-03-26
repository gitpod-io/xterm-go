package xterm

// Ported from xterm.js src/common/parser/ApcParser.ts.
//
// ApcParser handles Application Program Command sequences. Unlike OSC which
// uses numeric identifiers, APC uses the first character as the identifier
// (e.g., 'G' for Kitty graphics protocol).

// ApcHandler is the interface for handlers that process APC sequences.
// Intentionally separate from OscHandler to mirror xterm.js type structure.
type ApcHandler interface { //nolint:iface
	Start()
	Put(data []uint32, start, end int)
	End(success bool) bool
}

// ApcFallbackHandler is called when no registered handler matches the APC identifier.
type ApcFallbackHandler func(ident int, action string, payload ...interface{})

// ApcParser parses APC sequences and dispatches to registered handlers.
type ApcParser struct {
	state     ApcState
	active    []ApcHandler
	id        int
	handlers  map[int][]ApcHandler
	handlerFb ApcFallbackHandler
}

// NewApcParser creates a new ApcParser.
func NewApcParser() *ApcParser {
	return &ApcParser{
		state:     ApcStateStart,
		id:        -1,
		handlers:  make(map[int][]ApcHandler),
		handlerFb: func(int, string, ...interface{}) {},
	}
}

// RegisterHandler registers a handler for the given APC identifier (character code).
// Returns a Disposable that removes the handler when disposed.
func (p *ApcParser) RegisterHandler(ident int, handler ApcHandler) Disposable {
	p.handlers[ident] = append(p.handlers[ident], handler)
	handlerList := p.handlers[ident]
	return toDisposable(func() {
		for i, h := range handlerList {
			if h == handler {
				p.handlers[ident] = append(handlerList[:i], handlerList[i+1:]...)
				return
			}
		}
	})
}

// ClearHandler removes all handlers for the given identifier.
func (p *ApcParser) ClearHandler(ident int) {
	delete(p.handlers, ident)
}

// SetHandlerFallback sets the fallback handler called when no handler matches.
func (p *ApcParser) SetHandlerFallback(handler ApcFallbackHandler) {
	p.handlerFb = handler
}

// Dispose removes all handlers and resets state.
func (p *ApcParser) Dispose() {
	p.handlers = make(map[int][]ApcHandler)
	p.handlerFb = func(int, string, ...interface{}) {}
	p.active = nil
}

// Reset forces cleanup of active handlers and resets parser state.
func (p *ApcParser) Reset() {
	if p.state == ApcStatePayload {
		for j := len(p.active) - 1; j >= 0; j-- {
			p.active[j].End(false)
		}
	}
	p.active = nil
	p.id = -1
	p.state = ApcStateStart
}

func (p *ApcParser) start() {
	if handlers, ok := p.handlers[p.id]; ok && len(handlers) > 0 {
		p.active = handlers
		for j := len(p.active) - 1; j >= 0; j-- {
			p.active[j].Start()
		}
	} else {
		p.active = nil
		p.handlerFb(p.id, "START")
	}
}

func (p *ApcParser) put(data []uint32, start, end int) {
	if len(p.active) == 0 {
		p.handlerFb(p.id, "PUT", utf32ToString(data, start, end))
	} else {
		for j := len(p.active) - 1; j >= 0; j-- {
			p.active[j].Put(data, start, end)
		}
	}
}

// Start begins a new APC sequence, resetting any leftover state.
func (p *ApcParser) Start() {
	p.Reset()
	p.state = ApcStateID
}

// Put feeds data to the current APC command. The first character is used as
// the identifier, and subsequent data is passed as payload.
func (p *ApcParser) Put(data []uint32, start, end int) {
	if p.state == ApcStateAbort {
		return
	}
	if p.state == ApcStateID {
		if start < end {
			p.id = int(data[start])
			start++
			p.state = ApcStatePayload
			p.start()
		}
	}
	if p.state == ApcStatePayload && end-start > 0 {
		p.put(data, start, end)
	}
}

// End signals the end of an APC sequence. success indicates whether the
// sequence terminated normally or was aborted.
func (p *ApcParser) End(success bool) {
	if p.state == ApcStateStart {
		return
	}
	if p.state != ApcStateAbort {
		// Early end in ID state means empty APC — invalid, just reset.
		if p.state == ApcStateID {
			p.active = nil
			p.id = -1
			p.state = ApcStateStart
			return
		}
		if len(p.active) == 0 {
			p.handlerFb(p.id, "END", success)
		} else {
			for j := len(p.active) - 1; j >= 0; j-- {
				if p.active[j].End(success) {
					for k := j - 1; k >= 0; k-- {
						p.active[k].End(false)
					}
					break
				}
			}
		}
	}
	p.active = nil
	p.id = -1
	p.state = ApcStateStart
}

// ApcStringHandler is a convenience wrapper that collects APC payload as a
// string and calls a callback function on End.
type ApcStringHandler struct {
	handler  func(data string) bool
	data     string
	hitLimit bool
}

// NewApcStringHandler creates an ApcStringHandler from a callback.
func NewApcStringHandler(handler func(data string) bool) *ApcStringHandler {
	return &ApcStringHandler{handler: handler}
}

// Start resets the string accumulator.
func (h *ApcStringHandler) Start() {
	h.data = ""
	h.hitLimit = false
}

// Put appends payload data as a string.
func (h *ApcStringHandler) Put(data []uint32, start, end int) {
	if h.hitLimit {
		return
	}
	h.data += utf32ToString(data, start, end)
	if len(h.data) > ParserPayloadLimit {
		h.data = ""
		h.hitLimit = true
	}
}

// End calls the handler callback if the sequence ended successfully.
func (h *ApcStringHandler) End(success bool) bool {
	ret := false
	if !h.hitLimit && success {
		ret = h.handler(h.data)
	}
	h.data = ""
	h.hitLimit = false
	return ret
}
