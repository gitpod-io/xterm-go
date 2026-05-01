package xterm

// Ported from xterm.js src/headless/Terminal.ts and src/common/CoreTerminal.ts.
// Top-level Terminal that wires all services together and provides the public API.

import "strings"

// Option configures a Terminal.
type Option func(*TerminalOptions)

// WithCols sets the number of columns.
func WithCols(cols int) Option {
	return func(o *TerminalOptions) { o.Cols = cols }
}

// WithRows sets the number of rows.
func WithRows(rows int) Option {
	return func(o *TerminalOptions) { o.Rows = rows }
}

// WithScrollback sets the scrollback buffer size.
func WithScrollback(n int) Option {
	return func(o *TerminalOptions) { o.Scrollback = n }
}

// Terminal is a headless terminal emulator.
type Terminal struct {
	optionsService *OptionsService
	bufferService  *BufferService
	charsetService *CharsetService
	coreService    *CoreService
	oscLinkService *OscLinkService
	unicodeService *UnicodeService
	inputHandler   *InputHandler

	// Public event emitters (forwarded from sub-components).
	OnBellEmitter           EventEmitter[struct{}]
	OnTitleChangeEmitter    EventEmitter[string]
	OnIconNameChangeEmitter EventEmitter[string]
	OnLineFeedEmitter       EventEmitter[struct{}]
	OnCursorMoveEmitter     EventEmitter[struct{}]
	OnResizeEmitter         EventEmitter[BufferResizeEvent]
	OnScrollEmitter         EventEmitter[int]
	OnRenderEmitter         EventEmitter[RowRange]
}

// New creates a new Terminal with the given options.
func New(opts ...Option) *Terminal {
	termOpts := DefaultOptions()
	for _, fn := range opts {
		fn(&termOpts)
	}

	optsSvc := NewOptionsService(&termOpts)
	bufSvc := NewBufferService(optsSvc)
	charSvc := NewCharsetService()
	coreSvc := NewCoreService(optsSvc)
	oscLinkSvc := NewOscLinkService(bufSvc)
	uniSvc := NewUnicodeService()
	ih := NewInputHandler(bufSvc, charSvc, coreSvc, optsSvc, oscLinkSvc, uniSvc)

	t := &Terminal{
		optionsService: optsSvc,
		bufferService:  bufSvc,
		charsetService: charSvc,
		coreService:    coreSvc,
		oscLinkService: oscLinkSvc,
		unicodeService: uniSvc,
		inputHandler:   ih,
	}

	// Forward input handler events.
	ih.OnRequestBellEmitter.Event(func(struct{}) { t.OnBellEmitter.Fire(struct{}{}) })
	ih.OnTitleChangeEmitter.Event(func(s string) { t.OnTitleChangeEmitter.Fire(s) })
	ih.OnIconNameChangeEmitter.Event(func(s string) { t.OnIconNameChangeEmitter.Fire(s) })
	ih.OnLineFeedEmitter.Event(func(struct{}) { t.OnLineFeedEmitter.Fire(struct{}{}) })
	ih.OnCursorMoveEmitter.Event(func(struct{}) { t.OnCursorMoveEmitter.Fire(struct{}{}) })
	ih.OnRequestRefreshRowsEmitter.Event(func(r RowRange) { t.OnRenderEmitter.Fire(r) })

	// Forward buffer service events.
	bufSvc.OnResizeEmitter.Event(func(e BufferResizeEvent) { t.OnResizeEmitter.Fire(e) })
	bufSvc.OnScrollEmitter.Event(func(pos int) { t.OnScrollEmitter.Fire(pos) })

	// Forward core service data events (response data from DA, DSR, etc.).
	// No additional wiring needed — coreService.OnDataEmitter is the canonical source.

	// Handle reset requests from input handler (ESC c).
	ih.OnRequestResetEmitter.Event(func(struct{}) { t.Reset() })

	return t
}

// Write writes data to the terminal, implementing io.Writer.
func (t *Terminal) Write(p []byte) (n int, err error) {
	t.inputHandler.Parse(p)
	return len(p), nil
}

// WriteString writes a string to the terminal.
func (t *Terminal) WriteString(s string) {
	t.inputHandler.ParseString(s)
}

// Resize changes the terminal dimensions.
func (t *Terminal) Resize(cols, rows int) {
	if cols < MinimumCols {
		cols = MinimumCols
	}
	if rows < MinimumRows {
		rows = MinimumRows
	}
	if cols == t.bufferService.Cols && rows == t.bufferService.Rows {
		return
	}
	t.optionsService.Options.Cols = cols
	t.optionsService.Options.Rows = rows
	t.bufferService.Resize(cols, rows)
}

// Reset performs a full terminal reset.
func (t *Terminal) Reset() {
	t.optionsService.Options.Rows = t.bufferService.Rows
	t.optionsService.Options.Cols = t.bufferService.Cols
	t.inputHandler.Reset()
	t.bufferService.Reset()
	t.charsetService.Reset()
	t.coreService.Reset()
}

// Cols returns the number of columns.
func (t *Terminal) Cols() int { return t.bufferService.Cols }

// Rows returns the number of rows.
func (t *Terminal) Rows() int { return t.bufferService.Rows }

// CursorX returns the cursor column (0-based).
func (t *Terminal) CursorX() int { return t.bufferService.Buffer().X }

// CursorY returns the cursor row (0-based, relative to viewport).
func (t *Terminal) CursorY() int { return t.bufferService.Buffer().Y }

// Buffer returns the active buffer (for advanced access).
func (t *Terminal) Buffer() *Buffer { return t.bufferService.Buffer() }

// GetLine returns the content of a viewport line as a string.
// Returns "" if y is out of range.
func (t *Terminal) GetLine(y int) string {
	buf := t.bufferService.Buffer()
	if y < 0 || y >= t.bufferService.Rows {
		return ""
	}
	line := buf.Lines.Get(buf.YBase + y)
	if line == nil {
		return ""
	}
	return line.TranslateToString(true, 0, -1)
}

// String returns the entire visible viewport as a string.
// Trailing blank lines are trimmed. Each line has trailing whitespace trimmed.
func (t *Terminal) String() string {
	rows := t.bufferService.Rows
	lines := make([]string, rows)
	for i := range rows {
		lines[i] = t.GetLine(i)
	}
	// Trim trailing empty lines.
	last := rows - 1
	for last >= 0 && lines[last] == "" {
		last--
	}
	return strings.Join(lines[:last+1], "\n")
}

// OnData registers a callback for data sent from the terminal (e.g. DA responses).
func (t *Terminal) OnData(fn func(string)) Disposable {
	return t.coreService.OnDataEmitter.Event(fn)
}

// OnBell registers a callback for bell events.
func (t *Terminal) OnBell(fn func()) Disposable {
	return t.OnBellEmitter.Event(func(struct{}) { fn() })
}

// OnTitleChange registers a callback for title change events.
func (t *Terminal) OnTitleChange(fn func(string)) Disposable {
	return t.OnTitleChangeEmitter.Event(fn)
}

// IconName returns the current icon name set via OSC 1 or OSC 0.
func (t *Terminal) IconName() string { return t.inputHandler.iconName }

// OnIconNameChange registers a callback for icon name change events.
func (t *Terminal) OnIconNameChange(fn func(string)) Disposable {
	return t.OnIconNameChangeEmitter.Event(fn)
}

// OnLineFeed registers a callback for line feed events.
func (t *Terminal) OnLineFeed(fn func()) Disposable {
	return t.OnLineFeedEmitter.Event(func(struct{}) { fn() })
}

// OnCursorMove registers a callback for cursor move events.
func (t *Terminal) OnCursorMove(fn func()) Disposable {
	return t.OnCursorMoveEmitter.Event(func(struct{}) { fn() })
}

// OnResize registers a callback for terminal resize events.
func (t *Terminal) OnResize(fn func(BufferResizeEvent)) Disposable {
	return t.OnResizeEmitter.Event(fn)
}

// OnScroll registers a callback for scroll events.
func (t *Terminal) OnScroll(fn func(int)) Disposable {
	return t.OnScrollEmitter.Event(fn)
}

// OnRender registers a callback fired when terminal rows are dirty.
func (t *Terminal) OnRender(fn func(RowRange)) Disposable {
	return t.OnRenderEmitter.Event(fn)
}

// RegisterApcHandler registers a handler for APC escape sequences.
// ident is the character code of the first byte after ESC _ (e.g., 0x47 for 'G').
func (t *Terminal) RegisterApcHandler(ident int, handler func(data string) bool) Disposable {
	return t.inputHandler.parser.RegisterApcHandler(ident, NewApcStringHandler(handler))
}

// NormalBuffer returns the normal (primary) buffer.
func (t *Terminal) NormalBuffer() *Buffer { return t.bufferService.Buffers.Normal() }

// AltBuffer returns the alternate buffer.
func (t *Terminal) AltBuffer() *Buffer { return t.bufferService.Buffers.Alt() }

// IsAltBufferActive returns true if the alternate buffer is active.
func (t *Terminal) IsAltBufferActive() bool {
	return t.bufferService.Buffer() == t.bufferService.Buffers.Alt()
}

// CurAttrData returns the current cursor attribute data from the input handler.
func (t *Terminal) CurAttrData() AttributeData { return t.inputHandler.curAttrData }

// Modes returns the current ANSI modes.
func (t *Terminal) Modes() Modes { return t.coreService.Modes }

// DecPrivateModes returns the current DEC private modes.
func (t *Terminal) DecPrivateModes() DecPrivateModes { return t.coreService.DecPrivateModes }

// IsCursorHidden returns whether the cursor is hidden (DECTCEM).
func (t *Terminal) IsCursorHidden() bool { return t.coreService.IsCursorHidden }

// ScrollTop returns the top of the scroll region (0-based).
func (t *Terminal) ScrollTop() int { return t.bufferService.Buffer().ScrollTop }

// ScrollBottom returns the bottom of the scroll region (0-based).
func (t *Terminal) ScrollBottom() int { return t.bufferService.Buffer().ScrollBottom }

// Scrollback returns the scrollback buffer size.
func (t *Terminal) Scrollback() int { return t.optionsService.Options.Scrollback }

// Dispose cleans up all resources.
func (t *Terminal) Dispose() {
	t.inputHandler.Dispose()
	t.coreService.Dispose()
	t.OnBellEmitter.Dispose()
	t.OnTitleChangeEmitter.Dispose()
	t.OnIconNameChangeEmitter.Dispose()
	t.OnLineFeedEmitter.Dispose()
	t.OnCursorMoveEmitter.Dispose()
	t.OnResizeEmitter.Dispose()
	t.OnScrollEmitter.Dispose()
	t.OnRenderEmitter.Dispose()
}
