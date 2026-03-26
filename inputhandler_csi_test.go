package xterm

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// newTestInputHandler creates an InputHandler with default services for testing.
func newTestInputHandler(cols, rows int) *InputHandler {
	opts := DefaultOptions()
	opts.Cols = cols
	opts.Rows = rows
	opts.Scrollback = 1000
	optsSvc := NewOptionsService(&opts)
	bufSvc := NewBufferService(optsSvc)
	charSvc := NewCharsetService()
	coreSvc := NewCoreService(optsSvc)
	oscLinkSvc := NewOscLinkService(bufSvc)
	uniSvc := NewUnicodeService()
	return NewInputHandler(bufSvc, charSvc, coreSvc, optsSvc, oscLinkSvc, uniSvc)
}

func TestCursorPosition(t *testing.T) {
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
		{"move_to_1_1", "\x1b[1;1H", Expectation{0, 0}},
		{"move_to_5_10", "\x1b[5;10H", Expectation{9, 4}},
		{"default_is_1_1", "\x1b[H", Expectation{0, 0}},
		{"clamp_to_bounds", "\x1b[999;999H", Expectation{79, 23}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Input)
			buf := h.activeBuffer()
			got := Expectation{X: buf.X, Y: buf.Y}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestCursorMovement(t *testing.T) {
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
		{"CUU_up_1", "\x1b[5;5H\x1b[A", Expectation{4, 3}},
		{"CUU_up_3", "\x1b[5;5H\x1b[3A", Expectation{4, 1}},
		{"CUU_clamp_top", "\x1b[2;5H\x1b[99A", Expectation{4, 0}},
		{"CUD_down_1", "\x1b[5;5H\x1b[B", Expectation{4, 5}},
		{"CUD_down_3", "\x1b[5;5H\x1b[3B", Expectation{4, 7}},
		{"CUD_clamp_bottom", "\x1b[5;5H\x1b[99B", Expectation{4, 23}},
		{"CUF_forward_1", "\x1b[5;5H\x1b[C", Expectation{5, 4}},
		{"CUF_forward_5", "\x1b[5;5H\x1b[5C", Expectation{9, 4}},
		{"CUF_clamp_right", "\x1b[5;5H\x1b[999C", Expectation{79, 4}},
		{"CUB_backward_1", "\x1b[5;5H\x1b[D", Expectation{3, 4}},
		{"CUB_backward_3", "\x1b[5;5H\x1b[3D", Expectation{1, 4}},
		{"CUB_clamp_left", "\x1b[5;5H\x1b[999D", Expectation{0, 4}},
		{"CNL_next_line", "\x1b[5;5H\x1b[E", Expectation{0, 5}},
		{"CPL_preceding_line", "\x1b[5;5H\x1b[F", Expectation{0, 3}},
		{"CHA_column", "\x1b[5;5H\x1b[10G", Expectation{9, 4}},
		{"VPA_row", "\x1b[5;5H\x1b[10d", Expectation{4, 9}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Input)
			buf := h.activeBuffer()
			got := Expectation{X: buf.X, Y: buf.Y}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEraseInDisplay(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		LineContent string
		Err         string
	}
	tests := []struct {
		Name     string
		Setup    string // write some content first
		Erase    string // erase sequence
		CheckRow int    // which row to check
		Expected Expectation
	}{
		{
			Name:     "ED_0_erase_below",
			Setup:    "AAAA\x1b[2;1HBBBB\x1b[3;1HCCCC",
			Erase:    "\x1b[2;3H\x1b[0J", // cursor at row 2, col 3, erase below
			CheckRow: 2,                  // row 3 (0-indexed) should be blank
			Expected: Expectation{LineContent: ""},
		},
		{
			Name:     "ED_1_erase_above",
			Setup:    "AAAA\x1b[2;1HBBBB\x1b[3;1HCCCC",
			Erase:    "\x1b[2;3H\x1b[1J", // erase above
			CheckRow: 0,                  // row 1 should be blank
			Expected: Expectation{LineContent: ""},
		},
		{
			Name:     "ED_2_erase_all",
			Setup:    "AAAA\x1b[2;1HBBBB",
			Erase:    "\x1b[2J",
			CheckRow: 0,
			Expected: Expectation{LineContent: ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Setup)
			h.ParseString(tc.Erase)
			buf := h.activeBuffer()
			line := buf.Lines.Get(buf.YBase + tc.CheckRow)
			var got Expectation
			if line != nil {
				got.LineContent = line.TranslateToString(true, 0, -1)
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestEraseInLine(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		LineContent string
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			Name:     "EL_0_erase_right",
			Input:    "ABCDEFGH\x1b[1;4H\x1b[0K",
			Expected: Expectation{LineContent: "ABC"},
		},
		{
			Name:     "EL_1_erase_left",
			Input:    "ABCDEFGH\x1b[1;4H\x1b[1K",
			Expected: Expectation{LineContent: "    EFGH"},
		},
		{
			Name:     "EL_2_erase_all",
			Input:    "ABCDEFGH\x1b[1;4H\x1b[2K",
			Expected: Expectation{LineContent: ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Input)
			buf := h.activeBuffer()
			line := buf.Lines.Get(buf.YBase)
			var got Expectation
			if line != nil {
				got.LineContent = line.TranslateToString(true, 0, -1)
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

// TestEraseInLineBCE verifies that EL (Erase in Line) fills erased cells with the
// current background color (Back Color Erase).
func TestEraseInLineBCE(t *testing.T) {
	t.Parallel()
	h := newTestInputHandler(80, 24)
	// Set green background (SGR 42), write text, move cursor back, erase right.
	h.ParseString("\x1b[42mABCDEFGH\x1b[1;4H\x1b[0K")

	buf := h.activeBuffer()
	line := buf.Lines.Get(buf.YBase)
	if line == nil {
		t.Fatal("expected line")
	}
	// Cells 0-2 ("ABC") should have green bg from when they were written.
	// Cells 3+ should have green bg from BCE (erase inherits current bg).
	cell := NewCellData()
	for col := 3; col < 8; col++ {
		line.LoadCell(col, cell)
		if !cell.IsBgPalette() || cell.GetBgColor() != 2 {
			t.Errorf("cell[%d]: expected green bg (palette 2), got bgMode=0x%x bgColor=%d",
				col, cell.GetBgColorMode(), cell.GetBgColor())
		}
	}
}

// TestEraseInDisplayBCE verifies that ED (Erase in Display) fills erased cells with
// the current background color.
func TestEraseInDisplayBCE(t *testing.T) {
	t.Parallel()
	h := newTestInputHandler(80, 24)
	// Set red background, write content, then erase display.
	h.ParseString("\x1b[41mHello\x1b[2J")

	buf := h.activeBuffer()
	line := buf.Lines.Get(buf.YBase)
	if line == nil {
		t.Fatal("expected line")
	}
	cell := NewCellData()
	line.LoadCell(0, cell)
	if !cell.IsBgPalette() || cell.GetBgColor() != 1 {
		t.Errorf("cell[0]: expected red bg (palette 1), got bgMode=0x%x bgColor=%d",
			cell.GetBgColorMode(), cell.GetBgColor())
	}
}

func TestInsertDeleteChars(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		LineContent string
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			Name:     "ICH_insert_2",
			Input:    "ABCDEF\x1b[1;3H\x1b[2@",
			Expected: Expectation{LineContent: "AB  CDEF"},
		},
		{
			Name:     "DCH_delete_2",
			Input:    "ABCDEF\x1b[1;3H\x1b[2P",
			Expected: Expectation{LineContent: "ABEF"},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Input)
			buf := h.activeBuffer()
			line := buf.Lines.Get(buf.YBase)
			var got Expectation
			if line != nil {
				got.LineContent = line.TranslateToString(true, 0, -1)
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestInsertDeleteLines(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Row0 string
		Row1 string
		Row2 string
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			Name:     "IL_insert_1",
			Input:    "AAA\x1b[2;1HBBB\x1b[3;1HCCC\x1b[2;1H\x1b[1L",
			Expected: Expectation{Row0: "AAA", Row1: "", Row2: "BBB"},
		},
		{
			Name:     "DL_delete_1",
			Input:    "AAA\x1b[2;1HBBB\x1b[3;1HCCC\x1b[2;1H\x1b[1M",
			Expected: Expectation{Row0: "AAA", Row1: "CCC", Row2: ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Input)
			buf := h.activeBuffer()
			getRow := func(y int) string {
				line := buf.Lines.Get(buf.YBase + y)
				if line == nil {
					return ""
				}
				return line.TranslateToString(true, 0, -1)
			}
			got := Expectation{Row0: getRow(0), Row1: getRow(1), Row2: getRow(2)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestScrollUpDown(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Row0 string
		Row1 string
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			Name:     "SU_scroll_up_1",
			Input:    "AAA\x1b[2;1HBBB\x1b[1S",
			Expected: Expectation{Row0: "BBB", Row1: ""},
		},
		{
			Name:     "SD_scroll_down_1",
			Input:    "AAA\x1b[2;1HBBB\x1b[1T",
			Expected: Expectation{Row0: "", Row1: "AAA"},
		},
		{
			Name:     "SD_scroll_down_caret_alias",
			Input:    "AAA\x1b[2;1HBBB\x1b[1^",
			Expected: Expectation{Row0: "", Row1: "AAA"},
		},
		{
			Name:     "SD_scroll_down_caret_2",
			Input:    "AAA\x1b[2;1HBBB\x1b[2^",
			Expected: Expectation{Row0: "", Row1: ""},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Input)
			buf := h.activeBuffer()
			getRow := func(y int) string {
				line := buf.Lines.Get(buf.YBase + y)
				if line == nil {
					return ""
				}
				return line.TranslateToString(true, 0, -1)
			}
			got := Expectation{Row0: getRow(0), Row1: getRow(1)}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSetScrollRegion(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		ScrollTop    int
		ScrollBottom int
		X            int
		Y            int
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			Name:     "set_5_to_10",
			Input:    "\x1b[5;10r",
			Expected: Expectation{ScrollTop: 4, ScrollBottom: 9, X: 0, Y: 0},
		},
		{
			Name:     "default_full_screen",
			Input:    "\x1b[r",
			Expected: Expectation{ScrollTop: 0, ScrollBottom: 23, X: 0, Y: 0},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Input)
			buf := h.activeBuffer()
			got := Expectation{
				ScrollTop:    buf.ScrollTop,
				ScrollBottom: buf.ScrollBottom,
				X:            buf.X,
				Y:            buf.Y,
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSoftReset(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		ScrollTop    int
		ScrollBottom int
		Origin       bool
		InsertMode   bool
	}
	h := newTestInputHandler(80, 24)
	// Set some modes
	h.ParseString("\x1b[5;10r") // set scroll region
	h.ParseString("\x1b[?6h")   // origin mode
	h.ParseString("\x1b[4h")    // insert mode
	h.ParseString("\x1b[!p")    // soft reset
	buf := h.activeBuffer()
	got := Expectation{
		ScrollTop:    buf.ScrollTop,
		ScrollBottom: buf.ScrollBottom,
		Origin:       h.coreService.DecPrivateModes.Origin,
		InsertMode:   h.coreService.Modes.InsertMode,
	}
	expected := Expectation{
		ScrollTop:    0,
		ScrollBottom: 23,
		Origin:       false,
		InsertMode:   false,
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestSetResetMode(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		InsertMode            bool
		ApplicationCursorKeys bool
		Wraparound            bool
		BracketedPasteMode    bool
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			Name:     "set_insert_mode",
			Input:    "\x1b[4h",
			Expected: Expectation{InsertMode: true, Wraparound: true},
		},
		{
			Name:     "reset_insert_mode",
			Input:    "\x1b[4h\x1b[4l",
			Expected: Expectation{InsertMode: false, Wraparound: true},
		},
		{
			Name:     "DECSET_application_cursor_keys",
			Input:    "\x1b[?1h",
			Expected: Expectation{ApplicationCursorKeys: true, Wraparound: true},
		},
		{
			Name:     "DECRST_application_cursor_keys",
			Input:    "\x1b[?1h\x1b[?1l",
			Expected: Expectation{ApplicationCursorKeys: false, Wraparound: true},
		},
		{
			Name:     "DECSET_wraparound",
			Input:    "\x1b[?7h",
			Expected: Expectation{Wraparound: true},
		},
		{
			Name:     "DECRST_wraparound",
			Input:    "\x1b[?7l",
			Expected: Expectation{Wraparound: false},
		},
		{
			Name:     "DECSET_bracketed_paste",
			Input:    "\x1b[?2004h",
			Expected: Expectation{BracketedPasteMode: true, Wraparound: true},
		},
		{
			Name:     "DECRST_bracketed_paste",
			Input:    "\x1b[?2004h\x1b[?2004l",
			Expected: Expectation{BracketedPasteMode: false, Wraparound: true},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Input)
			got := Expectation{
				InsertMode:            h.coreService.Modes.InsertMode,
				ApplicationCursorKeys: h.coreService.DecPrivateModes.ApplicationCursorKeys,
				Wraparound:            h.coreService.DecPrivateModes.Wraparound,
				BracketedPasteMode:    h.coreService.DecPrivateModes.BracketedPasteMode,
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAltScreenBuffer(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		IsAlt bool
		Row0  string
	}
	h := newTestInputHandler(80, 24)
	h.ParseString("HELLO")
	// Switch to alt buffer (DECSET 1049) — saves cursor then switches
	h.ParseString("\x1b[?1049h")
	isAlt := h.bufferService.Buffer() == h.bufferService.Buffers.Alt()
	// Move to home position and write
	h.ParseString("\x1b[H" + "ALT")
	altRow := h.activeBuffer().Lines.Get(h.activeBuffer().YBase).TranslateToString(true, 0, -1)
	got := Expectation{IsAlt: isAlt, Row0: altRow}
	expected := Expectation{IsAlt: true, Row0: "ALT"}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("alt screen mismatch (-want +got):\n%s", diff)
	}

	// Switch back (DECRST 1049)
	h.ParseString("\x1b[?1049l")
	isNormal := h.bufferService.Buffer() == h.bufferService.Buffers.Normal()
	normalRow := h.activeBuffer().Lines.Get(h.activeBuffer().YBase).TranslateToString(true, 0, -1)
	got2 := Expectation{IsAlt: !isNormal, Row0: normalRow}
	expected2 := Expectation{IsAlt: false, Row0: "HELLO"}
	if diff := cmp.Diff(expected2, got2); diff != "" {
		t.Errorf("normal screen mismatch (-want +got):\n%s", diff)
	}
}

func TestDeviceStatus(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Response string
	}
	h := newTestInputHandler(80, 24)
	var response string
	h.coreService.OnDataEmitter.Event(func(data string) {
		response = data
	})
	// Move cursor to row 5, col 10 then query
	h.ParseString("\x1b[5;10H\x1b[6n")
	got := Expectation{Response: response}
	expected := Expectation{Response: "\x1b[5;10R"}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestDeviceAttributes(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Response string
	}
	h := newTestInputHandler(80, 24)
	var response string
	h.coreService.OnDataEmitter.Event(func(data string) {
		response = data
	})
	h.ParseString("\x1b[c")
	got := Expectation{Response: response}
	expected := Expectation{Response: "\x1b[?1;2c"}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestXtVersion(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Response string
	}

	t.Run("default param responds with DCS version", func(t *testing.T) {
		t.Parallel()
		h := newTestInputHandler(80, 24)
		var response string
		h.coreService.OnDataEmitter.Event(func(data string) {
			response = data
		})
		h.ParseString("\x1b[>q")
		got := Expectation{Response: response}
		expected := Expectation{Response: "\x1bP>|xterm-go(0.1.0)\x1b\\"}
		if diff := cmp.Diff(expected, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})

	t.Run("non-zero param produces no response", func(t *testing.T) {
		t.Parallel()
		h := newTestInputHandler(80, 24)
		var response string
		h.coreService.OnDataEmitter.Event(func(data string) {
			response = data
		})
		h.ParseString("\x1b[>1q")
		got := Expectation{Response: response}
		expected := Expectation{Response: ""}
		if diff := cmp.Diff(expected, got); diff != "" {
			t.Errorf("mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestEraseChars(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		LineContent string
	}
	h := newTestInputHandler(80, 24)
	h.ParseString("ABCDEFGH")
	h.ParseString("\x1b[1;3H\x1b[3X") // erase 3 chars at col 3
	buf := h.activeBuffer()
	line := buf.Lines.Get(buf.YBase)
	got := Expectation{LineContent: line.TranslateToString(true, 0, -1)}
	expected := Expectation{LineContent: "AB   FGH"}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestTabClear(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		TabsEmpty bool
	}
	h := newTestInputHandler(80, 24)
	// Clear all tabs
	h.ParseString("\x1b[3g")
	buf := h.activeBuffer()
	got := Expectation{TabsEmpty: len(buf.Tabs) == 0}
	expected := Expectation{TabsEmpty: true}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("mismatch (-want +got):\n%s", diff)
	}
}

func TestCursorHideShow(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Hidden bool
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{"hide_cursor", "\x1b[?25l", Expectation{Hidden: true}},
		{"show_cursor", "\x1b[?25l\x1b[?25h", Expectation{Hidden: false}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Input)
			got := Expectation{Hidden: h.coreService.IsCursorHidden}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSetCursorStyle(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Style *CursorStyle
	}
	block := CursorStyleBlock
	underline := CursorStyleUnderline
	bar := CursorStyleBar
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{"block", "\x1b[2 q", Expectation{Style: &block}},
		{"underline", "\x1b[4 q", Expectation{Style: &underline}},
		{"bar", "\x1b[6 q", Expectation{Style: &bar}},
		{"reset", "\x1b[2 q\x1b[0 q", Expectation{Style: nil}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Input)
			got := Expectation{Style: h.coreService.DecPrivateModes.CursorStyle}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSelectProtected(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Protected bool
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{"set_protected", "\x1b[1\"q", Expectation{Protected: true}},
		{"clear_protected", "\x1b[1\"q\x1b[2\"q", Expectation{Protected: false}},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Input)
			got := Expectation{Protected: h.curAttrData.Bg&BgFlagProtected != 0}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSetResetModeColorSchemeAndWin32(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		ColorSchemeUpdates bool
		Win32InputMode     bool
	}
	tests := []struct {
		Name     string
		Input    string
		Expected Expectation
	}{
		{
			Name:     "DECSET_color_scheme_updates",
			Input:    "\x1b[?2031h",
			Expected: Expectation{ColorSchemeUpdates: true},
		},
		{
			Name:     "DECRST_color_scheme_updates",
			Input:    "\x1b[?2031h\x1b[?2031l",
			Expected: Expectation{ColorSchemeUpdates: false},
		},
		{
			Name:     "DECSET_win32_input_mode",
			Input:    "\x1b[?9001h",
			Expected: Expectation{Win32InputMode: true},
		},
		{
			Name:     "DECRST_win32_input_mode",
			Input:    "\x1b[?9001h\x1b[?9001l",
			Expected: Expectation{Win32InputMode: false},
		},
		{
			Name:     "DECSET_both_modes",
			Input:    "\x1b[?2031h\x1b[?9001h",
			Expected: Expectation{ColorSchemeUpdates: true, Win32InputMode: true},
		},
		{
			Name:     "DECRST_both_modes",
			Input:    "\x1b[?2031h\x1b[?9001h\x1b[?2031l\x1b[?9001l",
			Expected: Expectation{ColorSchemeUpdates: false, Win32InputMode: false},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			h.ParseString(tc.Input)
			got := Expectation{
				ColorSchemeUpdates: h.coreService.DecPrivateModes.ColorSchemeUpdates,
				Win32InputMode:     h.coreService.DecPrivateModes.Win32InputMode,
			}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestKittyKeyboardFlagSwapOnAltBuffer(t *testing.T) {
	t.Parallel()

	type KKState struct {
		Flags     int
		MainFlags int
		AltFlags  int
	}

	tests := []struct {
		Name     string
		Setup    func(h *InputHandler)
		Input    string
		Expected KKState
	}{
		{
			Name: "DECSET_1049_saves_main_flags_restores_alt",
			Setup: func(h *InputHandler) {
				h.coreService.KittyKeyboard.Flags = 5
				h.coreService.KittyKeyboard.AltFlags = 2
			},
			Input:    "\x1b[?1049h",
			Expected: KKState{Flags: 2, MainFlags: 5, AltFlags: 2},
		},
		{
			Name: "DECSET_47_saves_main_flags_restores_alt",
			Setup: func(h *InputHandler) {
				h.coreService.KittyKeyboard.Flags = 3
				h.coreService.KittyKeyboard.AltFlags = 1
			},
			Input:    "\x1b[?47h",
			Expected: KKState{Flags: 1, MainFlags: 3, AltFlags: 1},
		},
		{
			Name: "DECSET_1047_saves_main_flags_restores_alt",
			Setup: func(h *InputHandler) {
				h.coreService.KittyKeyboard.Flags = 7
				h.coreService.KittyKeyboard.AltFlags = 0
			},
			Input:    "\x1b[?1047h",
			Expected: KKState{Flags: 0, MainFlags: 7, AltFlags: 0},
		},
		{
			Name: "DECRST_1049_saves_alt_flags_restores_main",
			Setup: func(h *InputHandler) {
				// First switch to alt buffer
				h.coreService.KittyKeyboard.Flags = 5
				h.ParseString("\x1b[?1049h")
				// Now set flags as if app changed them on alt screen
				h.coreService.KittyKeyboard.Flags = 9
			},
			Input:    "\x1b[?1049l",
			Expected: KKState{Flags: 5, MainFlags: 5, AltFlags: 9},
		},
		{
			Name: "DECRST_47_saves_alt_flags_restores_main",
			Setup: func(h *InputHandler) {
				h.coreService.KittyKeyboard.Flags = 4
				h.ParseString("\x1b[?47h")
				h.coreService.KittyKeyboard.Flags = 6
			},
			Input:    "\x1b[?47l",
			Expected: KKState{Flags: 4, MainFlags: 4, AltFlags: 6},
		},
		{
			Name: "DECRST_1047_saves_alt_flags_restores_main",
			Setup: func(h *InputHandler) {
				h.coreService.KittyKeyboard.Flags = 8
				h.ParseString("\x1b[?1047h")
				h.coreService.KittyKeyboard.Flags = 3
			},
			Input:    "\x1b[?1047l",
			Expected: KKState{Flags: 8, MainFlags: 8, AltFlags: 3},
		},
		{
			Name: "round_trip_preserves_flags",
			Setup: func(h *InputHandler) {
				h.coreService.KittyKeyboard.Flags = 10
				h.coreService.KittyKeyboard.AltFlags = 20
			},
			Input:    "\x1b[?1049h\x1b[?1049l",
			Expected: KKState{Flags: 10, MainFlags: 10, AltFlags: 20},
		},
	}

	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			h := newTestInputHandler(80, 24)
			if tc.Setup != nil {
				tc.Setup(h)
			}
			h.ParseString(tc.Input)
			kk := h.coreService.KittyKeyboard
			got := KKState{Flags: kk.Flags, MainFlags: kk.MainFlags, AltFlags: kk.AltFlags}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestKittyKeyboardStackSwapOnAltBuffer(t *testing.T) {
	t.Parallel()
	h := newTestInputHandler(80, 24)

	// Set up main stack
	h.coreService.KittyKeyboard.Flags = 1
	h.coreService.KittyKeyboard.MainStack = nil
	h.coreService.KittyKeyboard.AltStack = []int{10, 20}

	// Switch to alt buffer — main stack saved, alt stack restored
	h.ParseString("\x1b[?1049h")
	kk := h.coreService.KittyKeyboard
	if len(kk.AltStack) != 0 {
		t.Errorf("expected AltStack to be swapped out (len 0), got len %d", len(kk.AltStack))
	}
	if len(kk.MainStack) != 2 || kk.MainStack[0] != 10 || kk.MainStack[1] != 20 {
		t.Errorf("expected MainStack [10 20], got %v", kk.MainStack)
	}

	// Switch back to normal buffer — stacks swap back
	h.ParseString("\x1b[?1049l")
	kk = h.coreService.KittyKeyboard
	if len(kk.MainStack) != 0 {
		t.Errorf("expected MainStack to be swapped back (len 0), got len %d", len(kk.MainStack))
	}
	if len(kk.AltStack) != 2 || kk.AltStack[0] != 10 || kk.AltStack[1] != 20 {
		t.Errorf("expected AltStack [10 20], got %v", kk.AltStack)
	}
}
