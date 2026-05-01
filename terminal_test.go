package xterm

import (
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func newTestTerminal(cols, rows int) *Terminal {
	return New(WithCols(cols), WithRows(rows), WithScrollback(1000))
}

func TestTerminalBasicTextOutput(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Line0 string
		Full  string
		CurX  int
		CurY  int
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			"hello_world",
			"Hello, World!",
			Expectation{"Hello, World!", "Hello, World!", 13, 0},
		},
		{
			"two_lines",
			"Line1\r\nLine2",
			Expectation{"Line1", "Line1\nLine2", 5, 1},
		},
		{
			"empty_terminal",
			"",
			Expectation{"", "", 0, 0},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			term := newTestTerminal(80, 24)
			term.WriteString(tc.Input)
			got := Expectation{
				Line0: term.GetLine(0),
				Full:  term.String(),
				CurX:  term.CursorX(),
				CurY:  term.CursorY(),
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTerminalCursorMovement(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		X int
		Y int
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{"cup_1_1", "\x1b[1;1H", Expectation{0, 0}},
		{"cup_5_10", "\x1b[5;10H", Expectation{9, 4}},
		{"cuf", "ABC\x1b[5C", Expectation{8, 0}},
		{"cub", "ABCDEF\x1b[3D", Expectation{3, 0}},
		{"cuu", "\x1b[5;1HABC\x1b[2A", Expectation{3, 2}},
		{"cud", "ABC\x1b[3B", Expectation{3, 3}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			term := newTestTerminal(80, 24)
			term.WriteString(tc.Input)
			got := Expectation{X: term.CursorX(), Y: term.CursorY()}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTerminalLineWrapping(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(10, 5)
	// Write 25 chars — should wrap across 3 lines on a 10-col terminal.
	term.WriteString("ABCDEFGHIJKLMNOPQRSTUVWXY")
	type Expectation struct {
		Line0 string
		Line1 string
		Line2 string
		CurX  int
		CurY  int
	}
	want := Expectation{
		Line0: "ABCDEFGHIJ",
		Line1: "KLMNOPQRST",
		Line2: "UVWXY",
		CurX:  5,
		CurY:  2,
	}
	got := Expectation{
		Line0: term.GetLine(0),
		Line1: term.GetLine(1),
		Line2: term.GetLine(2),
		CurX:  term.CursorX(),
		CurY:  term.CursorY(),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestTerminalScrolling(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(20, 5)
	// Write 8 lines into a 5-row terminal — first 3 should scroll into scrollback.
	for i := range 8 {
		term.WriteString(fmt.Sprintf("Line%d\r\n", i))
	}
	type Expectation struct {
		ViewLine0 string
		ViewLine1 string
		ViewLine2 string
		YBase     int
	}
	want := Expectation{
		ViewLine0: "Line4",
		ViewLine1: "Line5",
		ViewLine2: "Line6",
		YBase:     4,
	}
	got := Expectation{
		ViewLine0: term.GetLine(0),
		ViewLine1: term.GetLine(1),
		ViewLine2: term.GetLine(2),
		YBase:     term.Buffer().YBase,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestTerminalColorsSGR(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Chars   string
		IsBold  bool
		FgColor int
		BgColor int
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			"bold_red_fg",
			"\x1b[1;31mX",
			Expectation{"X", true, 1, -1},
		},
		{
			"green_bg",
			"\x1b[42mY",
			Expectation{"Y", false, -1, 2},
		},
		{
			"256_color_fg",
			"\x1b[38;5;200mZ",
			Expectation{"Z", false, 200, -1},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			term := newTestTerminal(80, 24)
			term.WriteString(tc.Input)
			buf := term.Buffer()
			line := buf.Lines.Get(buf.YBase)
			cell := NewCellData()
			line.LoadCell(0, cell)
			fgColor := -1
			if !cell.IsFgDefault() {
				fgColor = cell.GetFgColor()
			}
			bgColor := -1
			if !cell.IsBgDefault() {
				bgColor = cell.GetBgColor()
			}
			got := Expectation{
				Chars:   cell.GetChars(),
				IsBold:  cell.IsBold() != 0,
				FgColor: fgColor,
				BgColor: bgColor,
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTerminalErase(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Line0 string
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			"erase_to_end_of_line",
			"ABCDEF\x1b[1;4H\x1b[K",
			Expectation{"ABC"},
		},
		{
			"erase_to_start_of_line",
			"ABCDEF\x1b[1;4H\x1b[1K",
			Expectation{"    EF"},
		},
		{
			"erase_entire_line",
			"ABCDEF\x1b[1;4H\x1b[2K",
			Expectation{""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			term := newTestTerminal(80, 24)
			term.WriteString(tc.Input)
			got := Expectation{Line0: term.GetLine(0)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestTerminalAltScreen(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	// Write to normal buffer.
	term.WriteString("NormalText")
	// Switch to alt screen (DECSET 1049). This saves cursor and clears alt.
	term.WriteString("\x1b[?1049h")
	// Move to beginning and write to alt buffer.
	term.WriteString("\x1b[H")
	term.WriteString("AltText")

	type Expectation struct {
		AltLine0    string
		NormalLine0 string
	}
	altLine := term.GetLine(0)

	// Switch back to normal screen (DECRST 1049).
	term.WriteString("\x1b[?1049l")
	normalLine := term.GetLine(0)

	got := Expectation{AltLine0: altLine, NormalLine0: normalLine}
	want := Expectation{AltLine0: "AltText", NormalLine0: "NormalText"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestTerminalResize(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	term.WriteString("Hello")

	term.Resize(40, 10)

	type Expectation struct {
		Cols  int
		Rows  int
		Line0 string
	}
	want := Expectation{Cols: 40, Rows: 10, Line0: "Hello"}
	got := Expectation{
		Cols:  term.Cols(),
		Rows:  term.Rows(),
		Line0: term.GetLine(0),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestTerminalIOWriter(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	n, err := fmt.Fprintf(term, "count=%d", 42)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	type Expectation struct {
		N     int
		Line0 string
	}
	want := Expectation{N: 8, Line0: "count=42"}
	got := Expectation{N: n, Line0: term.GetLine(0)}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestTerminalReset(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	term.WriteString("Some text\r\nMore text")
	term.Reset()

	type Expectation struct {
		Line0 string
		CurX  int
		CurY  int
	}
	want := Expectation{Line0: "", CurX: 0, CurY: 0}
	got := Expectation{
		Line0: term.GetLine(0),
		CurX:  term.CursorX(),
		CurY:  term.CursorY(),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestTerminalTitleChange(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	var title string
	term.OnTitleChange(func(s string) { title = s })
	// OSC 2 ; <title> ST
	term.WriteString("\x1b]2;My Terminal\x1b\\")
	if title != "My Terminal" {
		t.Errorf("title = %q, want %q", title, "My Terminal")
	}
}

func TestTerminalIconNameChange(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	var iconName string
	term.OnIconNameChange(func(s string) { iconName = s })

	// OSC 1 ; <name> BEL
	term.WriteString("\x1b]1;my-icon\x07")
	if iconName != "my-icon" {
		t.Errorf("iconName = %q, want %q", iconName, "my-icon")
	}
	if term.IconName() != "my-icon" {
		t.Errorf("IconName() = %q, want %q", term.IconName(), "my-icon")
	}
}

func TestTerminalOSC0_SetsTitleAndIconName(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	var title, iconName string
	term.OnTitleChange(func(s string) { title = s })
	term.OnIconNameChange(func(s string) { iconName = s })

	// OSC 0 should set both.
	term.WriteString("\x1b]0;both\x07")
	if title != "both" {
		t.Errorf("title = %q, want %q", title, "both")
	}
	if iconName != "both" {
		t.Errorf("iconName = %q, want %q", iconName, "both")
	}
}

func TestTerminalBell(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	bellCount := 0
	term.OnBell(func() { bellCount++ })
	term.WriteString("\x07\x07")
	if bellCount != 2 {
		t.Errorf("bellCount = %d, want 2", bellCount)
	}
}

func TestTerminalDeviceAttributes(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	var responses []string
	term.OnData(func(s string) { responses = append(responses, s) })
	// DA1: CSI c
	term.WriteString("\x1b[c")
	if len(responses) == 0 {
		t.Fatal("expected DA1 response, got none")
	}
	// DA1 response should start with ESC [ ? and end with c.
	resp := responses[0]
	if !strings.HasPrefix(resp, "\x1b[?") || !strings.HasSuffix(resp, "c") {
		t.Errorf("unexpected DA1 response: %q", resp)
	}
}

func TestTerminalGetLineOutOfRange(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	term.WriteString("Hello")
	if got := term.GetLine(-1); got != "" {
		t.Errorf("GetLine(-1) = %q, want empty", got)
	}
	if got := term.GetLine(24); got != "" {
		t.Errorf("GetLine(24) = %q, want empty", got)
	}
}

func TestTerminalStringTrimsTrailingBlankLines(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	term.WriteString("OnlyLine")
	s := term.String()
	if strings.Contains(s, "\n") {
		t.Errorf("String() should not contain newlines for single-line content, got %q", s)
	}
	if s != "OnlyLine" {
		t.Errorf("String() = %q, want %q", s, "OnlyLine")
	}
}

func TestTerminalLineFeedEvent(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	lfCount := 0
	term.OnLineFeed(func() { lfCount++ })
	term.WriteString("A\nB\nC")
	if lfCount != 2 {
		t.Errorf("linefeed count = %d, want 2", lfCount)
	}
}

func TestTerminalResizeClampMinimum(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	term.Resize(0, 0)
	if term.Cols() != MinimumCols {
		t.Errorf("Cols() = %d, want %d", term.Cols(), MinimumCols)
	}
	if term.Rows() != MinimumRows {
		t.Errorf("Rows() = %d, want %d", term.Rows(), MinimumRows)
	}
}

func TestTerminalResizeNoop(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	resizeCount := 0
	term.OnResizeEmitter.Event(func(BufferResizeEvent) { resizeCount++ })
	term.Resize(80, 24) // same dimensions — should be a no-op
	if resizeCount != 0 {
		t.Errorf("resize event fired %d times for no-op resize", resizeCount)
	}
}

func TestTerminalWriteBytes(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	n, err := term.Write([]byte("bytes"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 5 {
		t.Errorf("Write returned n=%d, want 5", n)
	}
	if got := term.GetLine(0); got != "bytes" {
		t.Errorf("GetLine(0) = %q, want %q", got, "bytes")
	}
}

func TestTerminalDefaultDimensions(t *testing.T) {
	t.Parallel()
	term := New() // no options — should use defaults (80x24)
	if term.Cols() != 80 {
		t.Errorf("Cols() = %d, want 80", term.Cols())
	}
	if term.Rows() != 24 {
		t.Errorf("Rows() = %d, want 24", term.Rows())
	}
}

func TestTerminalDispose(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	term.WriteString("Hello")
	term.Dispose() // should not panic
}

func TestTerminalEraseDisplay(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	term.WriteString("Line0\r\nLine1\r\nLine2")
	// ED 2 — erase entire display
	term.WriteString("\x1b[2J")
	for y := range 3 {
		if got := term.GetLine(y); got != "" {
			t.Errorf("GetLine(%d) = %q after ED2, want empty", y, got)
		}
	}
}

func TestTerminalCursorSaveRestore(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	term.WriteString("\x1b[5;10H") // move to row 5, col 10
	term.WriteString("\x1b7")      // save cursor (DECSC)
	term.WriteString("\x1b[1;1H")  // move to 1,1
	term.WriteString("\x1b8")      // restore cursor (DECRC)
	type Expectation struct {
		X int
		Y int
	}
	want := Expectation{X: 9, Y: 4}
	got := Expectation{X: term.CursorX(), Y: term.CursorY()}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestTerminalScrollRegion(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(20, 5)
	// Set scroll region to rows 2-4 (1-indexed: DECSTBM)
	term.WriteString("\x1b[2;4r")
	// Move to row 4 and write lines to trigger scrolling within region
	term.WriteString("\x1b[2;1H")
	term.WriteString("R2\r\nR3\r\nR4\r\nR5")
	// Row 1 (index 0) should be untouched
	if got := term.GetLine(0); got != "" {
		t.Errorf("GetLine(0) = %q, want empty (outside scroll region)", got)
	}
}

func TestTerminalOnRender(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	var renders []RowRange
	term.OnRender(func(r RowRange) { renders = append(renders, r) })
	term.WriteString("Hello")
	if len(renders) == 0 {
		t.Fatal("expected OnRender to fire, got no events")
	}
	// The render event should cover row 0 (where "Hello" was written).
	found := false
	for _, r := range renders {
		if r.Start <= 0 && r.End >= 0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected render event covering row 0, got %v", renders)
	}
}

func TestTerminalOnRenderDispose(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	count := 0
	d := term.OnRender(func(RowRange) { count++ })
	term.WriteString("A")
	if count == 0 {
		t.Fatal("expected OnRender to fire")
	}
	first := count
	d.Dispose()
	term.WriteString("B")
	if count != first {
		t.Errorf("OnRender fired after Dispose: count went from %d to %d", first, count)
	}
}

func TestTerminalRegisterApcHandler(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	var received string
	// Register handler for APC identifier 'G' (0x47) — used by Kitty graphics protocol.
	term.RegisterApcHandler(0x47, func(data string) bool {
		received = data
		return true
	})
	// Send APC G <payload> ST: ESC _ G <payload> ESC backslash
	term.WriteString("\x1b_Ghello-apc\x1b\\")
	if received != "hello-apc" {
		t.Errorf("APC handler received %q, want %q", received, "hello-apc")
	}
}

func TestTerminalRegisterApcHandlerDispose(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	callCount := 0
	d := term.RegisterApcHandler(0x47, func(data string) bool {
		callCount++
		return true
	})
	term.WriteString("\x1b_Gfirst\x1b\\")
	if callCount != 1 {
		t.Fatalf("expected 1 call, got %d", callCount)
	}
	d.Dispose()
	term.WriteString("\x1b_Gsecond\x1b\\")
	if callCount != 1 {
		t.Errorf("APC handler called after Dispose: count = %d, want 1", callCount)
	}
}

func TestTerminalOnCursorMove(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	moveCount := 0
	term.OnCursorMove(func() { moveCount++ })
	// CUP (cursor position) triggers a cursor move event.
	term.WriteString("\x1b[5;10H")
	if moveCount == 0 {
		t.Error("expected OnCursorMove to fire, got 0 events")
	}
}

func TestTerminalOnCursorMoveDispose(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	count := 0
	d := term.OnCursorMove(func() { count++ })
	term.WriteString("\x1b[2;1H")
	if count == 0 {
		t.Fatal("expected OnCursorMove to fire")
	}
	first := count
	d.Dispose()
	term.WriteString("\x1b[3;1H")
	if count != first {
		t.Errorf("OnCursorMove fired after Dispose: count went from %d to %d", first, count)
	}
}

func TestTerminalOnResize(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	var events []BufferResizeEvent
	term.OnResize(func(e BufferResizeEvent) { events = append(events, e) })
	term.Resize(40, 10)
	if len(events) == 0 {
		t.Fatal("expected OnResize to fire, got no events")
	}
	got := events[len(events)-1]
	if got.Cols != 40 || got.Rows != 10 {
		t.Errorf("OnResize event = {Cols:%d, Rows:%d}, want {Cols:40, Rows:10}", got.Cols, got.Rows)
	}
}

func TestTerminalOnResizeDispose(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	count := 0
	d := term.OnResize(func(BufferResizeEvent) { count++ })
	term.Resize(40, 10)
	if count == 0 {
		t.Fatal("expected OnResize to fire")
	}
	first := count
	d.Dispose()
	term.Resize(60, 20)
	if count != first {
		t.Errorf("OnResize fired after Dispose: count went from %d to %d", first, count)
	}
}

func TestTerminalOnScroll(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(20, 5)
	var scrollPositions []int
	term.OnScroll(func(pos int) { scrollPositions = append(scrollPositions, pos) })
	// Write enough lines to trigger scrolling in a 5-row terminal.
	for i := range 8 {
		term.WriteString(fmt.Sprintf("Line%d\r\n", i))
	}
	if len(scrollPositions) == 0 {
		t.Fatal("expected OnScroll to fire, got no events")
	}
}

func TestTerminalOnScrollDispose(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(20, 5)
	count := 0
	d := term.OnScroll(func(int) { count++ })
	// Trigger scrolling.
	for i := range 8 {
		term.WriteString(fmt.Sprintf("Line%d\r\n", i))
	}
	if count == 0 {
		t.Fatal("expected OnScroll to fire")
	}
	first := count
	d.Dispose()
	// Write more to trigger additional scrolling.
	for i := range 5 {
		term.WriteString(fmt.Sprintf("More%d\r\n", i))
	}
	if count != first {
		t.Errorf("OnScroll fired after Dispose: count went from %d to %d", first, count)
	}
}

func TestTerminalTabStops(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)
	term.WriteString("A\tB")
	line := term.GetLine(0)
	// Default tab stop is 8, so 'A' at 0, tab to 8, 'B' at 8.
	if !strings.Contains(line, "A") || !strings.Contains(line, "B") {
		t.Errorf("GetLine(0) = %q, expected A and B with tab spacing", line)
	}
	if term.CursorX() != 9 {
		t.Errorf("CursorX() = %d, want 9 (after tab + B)", term.CursorX())
	}
}

func TestTerminalRegisterCsiHandler(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)

	var called bool
	var gotParams []int32
	d := term.RegisterCsiHandler(FunctionIdentifier{Final: 'Z'}, func(params *Params) bool {
		called = true
		gotParams = make([]int32, params.Length)
		copy(gotParams, params.Params[:params.Length])
		return true
	})

	// Send CSI 1;2 Z
	term.WriteString("\x1b[1;2Z")
	if !called {
		t.Fatal("CSI handler was not called")
	}
	if len(gotParams) < 2 || gotParams[0] != 1 || gotParams[1] != 2 {
		t.Fatalf("got params %v, want [1 2]", gotParams)
	}

	// Dispose and verify handler no longer fires.
	called = false
	d.Dispose()
	term.WriteString("\x1b[1;2Z")
	if called {
		t.Fatal("CSI handler was called after Dispose")
	}
}

func TestTerminalRegisterEscHandler(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)

	var called bool
	d := term.RegisterEscHandler(FunctionIdentifier{Intermediates: "#", Final: '9'}, func() bool {
		called = true
		return true
	})

	// Send ESC # 9
	term.WriteString("\x1b#9")
	if !called {
		t.Fatal("ESC handler was not called")
	}

	// Dispose and verify handler no longer fires.
	called = false
	d.Dispose()
	term.WriteString("\x1b#9")
	if called {
		t.Fatal("ESC handler was called after Dispose")
	}
}

func TestTerminalRegisterDcsHandler(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)

	var gotData string
	d := term.RegisterDcsHandler(FunctionIdentifier{Final: 'z'}, NewDcsStringHandler(func(data string, params *Params) bool {
		gotData = data
		return true
	}))

	// Send DCS z <payload> ST (DCS = ESC P, ST = ESC \)
	term.WriteString("\x1bPz" + "hello" + "\x1b\\")
	if gotData != "hello" {
		t.Fatalf("DCS handler got data %q, want %q", gotData, "hello")
	}

	// Dispose and verify handler no longer fires.
	gotData = ""
	d.Dispose()
	term.WriteString("\x1bPz" + "world" + "\x1b\\")
	if gotData != "" {
		t.Fatalf("DCS handler got data %q after Dispose, want empty", gotData)
	}
}

func TestTerminalRegisterOscHandler(t *testing.T) {
	t.Parallel()
	term := newTestTerminal(80, 24)

	var gotData string
	// Use a high OSC number unlikely to conflict with built-in handlers.
	d := term.RegisterOscHandler(9999, NewOscStringHandler(func(data string) bool {
		gotData = data
		return true
	}))

	// Send OSC 9999 ; payload BEL
	term.WriteString("\x1b]9999;test-payload\x07")
	if gotData != "test-payload" {
		t.Fatalf("OSC handler got data %q, want %q", gotData, "test-payload")
	}

	// Dispose and verify handler no longer fires.
	gotData = ""
	d.Dispose()
	term.WriteString("\x1b]9999;after-dispose\x07")
	if gotData != "" {
		t.Fatalf("OSC handler got data %q after Dispose, want empty", gotData)
	}
}

// fillScrollback writes enough lines to the terminal to create scrollback content.
func fillScrollback(term *Terminal, lineCount int) {
	for i := range lineCount {
		term.WriteString(fmt.Sprintf("line %d\r\n", i))
	}
}

func TestTerminalScrollLines(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		YDisp int
		YBase int
	}

	term := newTestTerminal(80, 5)
	fillScrollback(term, 20)

	yBase := term.Buffer().YBase

	// Scroll up 3 lines
	term.ScrollLines(-3)

	got := Expectation{
		YDisp: term.Buffer().YDisp,
		YBase: term.Buffer().YBase,
	}
	expected := Expectation{
		YDisp: yBase - 3,
		YBase: yBase,
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestTerminalScrollLinesDown(t *testing.T) {
	t.Parallel()

	term := newTestTerminal(80, 5)
	fillScrollback(term, 20)

	// Scroll up then back down
	term.ScrollLines(-5)
	term.ScrollLines(5)

	if term.Buffer().YDisp != term.Buffer().YBase {
		t.Errorf("YDisp = %d, want %d (YBase)", term.Buffer().YDisp, term.Buffer().YBase)
	}
}

func TestTerminalScrollPages(t *testing.T) {
	t.Parallel()

	term := newTestTerminal(80, 5)
	fillScrollback(term, 30)

	yBase := term.Buffer().YBase

	// Scroll up 2 pages (2 * 5 rows = 10 lines)
	term.ScrollPages(-2)

	expected := yBase - 10
	if term.Buffer().YDisp != expected {
		t.Errorf("YDisp = %d, want %d", term.Buffer().YDisp, expected)
	}
}

func TestTerminalScrollToTop(t *testing.T) {
	t.Parallel()

	term := newTestTerminal(80, 5)
	fillScrollback(term, 20)

	term.ScrollToTop()

	if term.Buffer().YDisp != 0 {
		t.Errorf("YDisp = %d, want 0", term.Buffer().YDisp)
	}
}

func TestTerminalScrollToBottom(t *testing.T) {
	t.Parallel()

	term := newTestTerminal(80, 5)
	fillScrollback(term, 20)

	// Scroll to top first, then back to bottom
	term.ScrollToTop()
	term.ScrollToBottom()

	if term.Buffer().YDisp != term.Buffer().YBase {
		t.Errorf("YDisp = %d, want %d (YBase)", term.Buffer().YDisp, term.Buffer().YBase)
	}
}

func TestTerminalScrollToLine(t *testing.T) {
	t.Parallel()

	term := newTestTerminal(80, 5)
	fillScrollback(term, 20)

	term.ScrollToLine(5)

	if term.Buffer().YDisp != 5 {
		t.Errorf("YDisp = %d, want 5", term.Buffer().YDisp)
	}
}

func TestTerminalScrollToLineClamps(t *testing.T) {
	t.Parallel()

	term := newTestTerminal(80, 5)
	fillScrollback(term, 20)

	// Scroll to a line beyond YBase — should clamp to YBase
	term.ScrollToLine(9999)

	if term.Buffer().YDisp != term.Buffer().YBase {
		t.Errorf("YDisp = %d, want %d (YBase)", term.Buffer().YDisp, term.Buffer().YBase)
	}
}

func TestTerminalScrollNoScrollback(t *testing.T) {
	t.Parallel()

	term := newTestTerminal(80, 5)
	// No scrollback content — scrolling should be a no-op
	term.ScrollLines(-5)

	if term.Buffer().YDisp != 0 {
		t.Errorf("YDisp = %d, want 0", term.Buffer().YDisp)
	}
}

func TestTerminalClear(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		YBase           int
		YDisp           int
		IsUserScrolling bool
	}

	term := newTestTerminal(80, 5)
	fillScrollback(term, 20)

	// Scroll up to simulate user scrolling
	term.ScrollToTop()

	term.Clear()

	got := Expectation{
		YBase:           term.Buffer().YBase,
		YDisp:           term.Buffer().YDisp,
		IsUserScrolling: term.bufferService.IsUserScrolling,
	}
	expected := Expectation{
		YBase:           0,
		YDisp:           0,
		IsUserScrolling: false,
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestTerminalClearEmptyTerminal(t *testing.T) {
	t.Parallel()

	term := newTestTerminal(80, 5)
	// Clear on empty terminal should be a no-op (no panic)
	term.Clear()

	if term.Buffer().YBase != 0 {
		t.Errorf("YBase = %d, want 0", term.Buffer().YBase)
	}
}

func TestTerminalClearFiresScrollEvent(t *testing.T) {
	t.Parallel()

	term := newTestTerminal(80, 5)
	fillScrollback(term, 20)

	scrollFired := false
	term.OnScroll(func(int) { scrollFired = true })

	term.Clear()

	if !scrollFired {
		t.Error("expected OnScroll to fire after Clear()")
	}
}

func TestTerminalAddMarker(t *testing.T) {
	t.Parallel()

	term := newTestTerminal(80, 24)
	term.WriteString("hello\r\nworld\r\n")

	// Cursor is at row 2 (0-based), add marker at cursor position
	marker := term.AddMarker(0)
	if marker == nil {
		t.Fatal("AddMarker returned nil")
	}

	expectedLine := term.Buffer().YBase + term.CursorY()
	if marker.Line != expectedLine {
		t.Errorf("marker.Line = %d, want %d", marker.Line, expectedLine)
	}
}

func TestTerminalAddMarkerWithOffset(t *testing.T) {
	t.Parallel()

	term := newTestTerminal(80, 24)
	term.WriteString("line1\r\nline2\r\nline3\r\n")

	// Add marker 1 line above cursor
	marker := term.AddMarker(-1)
	if marker == nil {
		t.Fatal("AddMarker returned nil")
	}

	expectedLine := term.Buffer().YBase + term.CursorY() - 1
	if marker.Line != expectedLine {
		t.Errorf("marker.Line = %d, want %d", marker.Line, expectedLine)
	}
}

func TestTerminalScrollLinesFiresEvent(t *testing.T) {
	t.Parallel()

	term := newTestTerminal(80, 5)
	fillScrollback(term, 20)

	scrollEvents := 0
	term.OnScroll(func(int) { scrollEvents++ })

	term.ScrollLines(-3)

	if scrollEvents != 1 {
		t.Errorf("scroll events = %d, want 1", scrollEvents)
	}
}


