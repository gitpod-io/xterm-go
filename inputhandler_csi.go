package xterm

import (
	"fmt"
)

// moveCursor moves the cursor by a relative offset, clamping to valid range.
func (h *InputHandler) moveCursor(x, y int) {
	h.restrictCursor()
	h.setCursor(h.activeBuffer().X+x, h.activeBuffer().Y+y)
}

// eraseInBufferLine erases cells in a buffer line.
func (h *InputHandler) eraseInBufferLine(y, start, end int, clearWrap bool, respectProtect bool) {
	buf := h.activeBuffer()
	line := buf.Lines.Get(buf.YBase + y)
	if line == nil {
		return
	}
	line.ReplaceCells(start, end, buf.GetNullCell(h.eraseAttrData()), respectProtect)
	if clearWrap {
		line.IsWrapped = false
	}
}

// resetBufferLine resets an entire buffer line.
func (h *InputHandler) resetBufferLine(y int, respectProtect bool) {
	buf := h.activeBuffer()
	line := buf.Lines.Get(buf.YBase + y)
	if line != nil {
		line.Fill(buf.GetNullCell(h.eraseAttrData()), respectProtect)
		buf.ClearMarkers(buf.YBase + y)
		line.IsWrapped = false
	}
}

// --- Cursor movement ---

func (h *InputHandler) cursorUp(params *Params) bool {
	buf := h.activeBuffer()
	diffToTop := buf.Y - buf.ScrollTop
	n := max(int(params.Params[0]), 1)
	if diffToTop >= 0 {
		h.moveCursor(0, -min(diffToTop, n))
	} else {
		h.moveCursor(0, -n)
	}
	return true
}

func (h *InputHandler) cursorDown(params *Params) bool {
	buf := h.activeBuffer()
	diffToBottom := buf.ScrollBottom - buf.Y
	n := max(int(params.Params[0]), 1)
	if diffToBottom >= 0 {
		h.moveCursor(0, min(diffToBottom, n))
	} else {
		h.moveCursor(0, n)
	}
	return true
}

func (h *InputHandler) cursorForward(params *Params) bool {
	h.moveCursor(max(int(params.Params[0]), 1), 0)
	return true
}

func (h *InputHandler) cursorBackward(params *Params) bool {
	h.moveCursor(-max(int(params.Params[0]), 1), 0)
	return true
}

func (h *InputHandler) cursorNextLine(params *Params) bool {
	h.cursorDown(params)
	h.activeBuffer().X = 0
	return true
}

func (h *InputHandler) cursorPrecedingLine(params *Params) bool {
	h.cursorUp(params)
	h.activeBuffer().X = 0
	return true
}

func (h *InputHandler) cursorCharAbsolute(params *Params) bool {
	h.setCursor(max(int(params.Params[0]), 1)-1, h.activeBuffer().Y)
	return true
}

func (h *InputHandler) cursorPosition(params *Params) bool {
	col := 0
	if params.Length >= 2 {
		col = max(int(params.Params[1]), 1) - 1
	}
	row := max(int(params.Params[0]), 1) - 1
	h.setCursor(col, row)
	return true
}

func (h *InputHandler) charPosAbsolute(params *Params) bool {
	h.setCursor(max(int(params.Params[0]), 1)-1, h.activeBuffer().Y)
	return true
}

func (h *InputHandler) hPositionRelative(params *Params) bool {
	h.moveCursor(max(int(params.Params[0]), 1), 0)
	return true
}

func (h *InputHandler) linePosAbsolute(params *Params) bool {
	h.setCursor(h.activeBuffer().X, max(int(params.Params[0]), 1)-1)
	return true
}

func (h *InputHandler) vPositionRelative(params *Params) bool {
	h.moveCursor(0, max(int(params.Params[0]), 1))
	return true
}

func (h *InputHandler) hVPosition(params *Params) bool {
	return h.cursorPosition(params)
}

// --- Tab ---

func (h *InputHandler) cursorForwardTab(params *Params) bool {
	buf := h.activeBuffer()
	if buf.X >= h.bufferService.Cols {
		return true
	}
	n := max(int(params.Params[0]), 1)
	for n > 0 {
		buf.X = buf.NextStop(buf.X)
		n--
	}
	return true
}

func (h *InputHandler) cursorBackwardTab(params *Params) bool {
	buf := h.activeBuffer()
	if buf.X >= h.bufferService.Cols {
		return true
	}
	n := max(int(params.Params[0]), 1)
	for n > 0 {
		buf.X = buf.PrevStop(buf.X)
		n--
	}
	return true
}

func (h *InputHandler) tabClear(params *Params) bool {
	buf := h.activeBuffer()
	switch p := params.Params[0]; p {
	case 0:
		delete(buf.Tabs, buf.X)
	case 3:
		buf.Tabs = make(map[int]bool)
	}
	return true
}

// --- Erase ---

func (h *InputHandler) eraseInDisplay(params *Params) bool {
	return h.eraseInDisplayInternal(params, false)
}

func (h *InputHandler) eraseInDisplayProtected(params *Params) bool {
	return h.eraseInDisplayInternal(params, true)
}

func (h *InputHandler) eraseInDisplayInternal(params *Params, respectProtect bool) bool {
	h.restrictCursor(h.bufferService.Cols)
	buf := h.activeBuffer()
	switch params.Params[0] {
	case 0: // erase below
		j := buf.Y
		h.dirtyRowTracker.MarkDirty(j)
		h.eraseInBufferLine(j, buf.X, h.bufferService.Cols, buf.X == 0, respectProtect)
		j++
		for ; j < h.bufferService.Rows; j++ {
			h.resetBufferLine(j, respectProtect)
		}
		h.dirtyRowTracker.MarkDirty(j - 1)
	case 1: // erase above
		j := buf.Y
		h.dirtyRowTracker.MarkDirty(j)
		h.eraseInBufferLine(j, 0, buf.X+1, true, respectProtect)
		if buf.X+1 >= h.bufferService.Cols {
			nextLine := buf.Lines.Get(buf.YBase + j + 1)
			if nextLine != nil {
				nextLine.IsWrapped = false
			}
		}
		for j > 0 {
			j--
			h.resetBufferLine(j, respectProtect)
		}
		h.dirtyRowTracker.MarkDirty(0)
	case 2: // erase all
		j := h.bufferService.Rows
		h.dirtyRowTracker.MarkDirty(j - 1)
		for j > 0 {
			j--
			h.resetBufferLine(j, respectProtect)
		}
		h.dirtyRowTracker.MarkDirty(0)
	case 3: // erase scrollback
		scrollBackSize := buf.Lines.Length() - h.bufferService.Rows
		if scrollBackSize > 0 {
			buf.Lines.TrimStart(scrollBackSize)
			buf.YBase = max(buf.YBase-scrollBackSize, 0)
			buf.YDisp = max(buf.YDisp-scrollBackSize, 0)
		}
	}
	return true
}

func (h *InputHandler) eraseInLine(params *Params) bool {
	return h.eraseInLineInternal(params, false)
}

func (h *InputHandler) eraseInLineProtected(params *Params) bool {
	return h.eraseInLineInternal(params, true)
}

func (h *InputHandler) eraseInLineInternal(params *Params, respectProtect bool) bool {
	h.restrictCursor(h.bufferService.Cols)
	buf := h.activeBuffer()
	switch params.Params[0] {
	case 0: // erase right
		h.eraseInBufferLine(buf.Y, buf.X, h.bufferService.Cols, buf.X == 0, respectProtect)
	case 1: // erase left
		h.eraseInBufferLine(buf.Y, 0, buf.X+1, false, respectProtect)
	case 2: // erase entire line
		h.eraseInBufferLine(buf.Y, 0, h.bufferService.Cols, true, respectProtect)
	}
	h.dirtyRowTracker.MarkDirty(buf.Y)
	return true
}

func (h *InputHandler) eraseChars(params *Params) bool {
	h.restrictCursor()
	buf := h.activeBuffer()
	line := buf.Lines.Get(buf.YBase + buf.Y)
	if line != nil {
		line.ReplaceCells(buf.X, buf.X+max(int(params.Params[0]), 1),
			buf.GetNullCell(h.eraseAttrData()), false)
		h.dirtyRowTracker.MarkDirty(buf.Y)
	}
	return true
}

// --- Insert / Delete ---

func (h *InputHandler) insertChars(params *Params) bool {
	h.restrictCursor()
	buf := h.activeBuffer()
	line := buf.Lines.Get(buf.YBase + buf.Y)
	if line != nil {
		line.InsertCells(buf.X, max(int(params.Params[0]), 1),
			buf.GetNullCell(h.eraseAttrData()))
		h.dirtyRowTracker.MarkDirty(buf.Y)
	}
	return true
}

func (h *InputHandler) deleteChars(params *Params) bool {
	h.restrictCursor()
	buf := h.activeBuffer()
	line := buf.Lines.Get(buf.YBase + buf.Y)
	if line != nil {
		line.DeleteCells(buf.X, max(int(params.Params[0]), 1),
			buf.GetNullCell(h.eraseAttrData()))
		h.dirtyRowTracker.MarkDirty(buf.Y)
	}
	return true
}

func (h *InputHandler) insertLines(params *Params) bool {
	h.restrictCursor()
	buf := h.activeBuffer()
	if buf.Y > buf.ScrollBottom || buf.Y < buf.ScrollTop {
		return true
	}
	n := max(int(params.Params[0]), 1)
	row := buf.YBase + buf.Y
	scrollBottomRowsOffset := h.bufferService.Rows - 1 - buf.ScrollBottom
	scrollBottomAbsolute := h.bufferService.Rows - 1 + buf.YBase - scrollBottomRowsOffset + 1
	for range n {
		buf.Lines.Splice(scrollBottomAbsolute-1, 1)
		buf.Lines.Splice(row, 0, buf.GetBlankLine(h.eraseAttrData(), false))
	}
	h.dirtyRowTracker.MarkRangeDirty(buf.Y, buf.ScrollBottom)
	buf.X = 0
	return true
}

func (h *InputHandler) deleteLines(params *Params) bool {
	h.restrictCursor()
	buf := h.activeBuffer()
	if buf.Y > buf.ScrollBottom || buf.Y < buf.ScrollTop {
		return true
	}
	n := max(int(params.Params[0]), 1)
	row := buf.YBase + buf.Y
	j := h.bufferService.Rows - 1 - buf.ScrollBottom
	j = h.bufferService.Rows - 1 + buf.YBase - j
	for range n {
		buf.Lines.Splice(row, 1)
		buf.Lines.Splice(j, 0, buf.GetBlankLine(h.eraseAttrData(), false))
	}
	h.dirtyRowTracker.MarkRangeDirty(buf.Y, buf.ScrollBottom)
	buf.X = 0
	return true
}

// --- Scroll ---

func (h *InputHandler) scrollUp(params *Params) bool {
	buf := h.activeBuffer()
	n := max(int(params.Params[0]), 1)
	for range n {
		buf.Lines.Splice(buf.YBase+buf.ScrollTop, 1)
		buf.Lines.Splice(buf.YBase+buf.ScrollBottom, 0, buf.GetBlankLine(h.eraseAttrData(), false))
	}
	h.dirtyRowTracker.MarkRangeDirty(buf.ScrollTop, buf.ScrollBottom)
	return true
}

func (h *InputHandler) scrollDown(params *Params) bool {
	buf := h.activeBuffer()
	n := max(int(params.Params[0]), 1)
	for range n {
		buf.Lines.Splice(buf.YBase+buf.ScrollBottom, 1)
		defAttr := DefaultAttrData()
		buf.Lines.Splice(buf.YBase+buf.ScrollTop, 0, buf.GetBlankLine(&defAttr, false))
	}
	h.dirtyRowTracker.MarkRangeDirty(buf.ScrollTop, buf.ScrollBottom)
	return true
}

func (h *InputHandler) scrollLeft(params *Params) bool {
	buf := h.activeBuffer()
	if buf.Y > buf.ScrollBottom || buf.Y < buf.ScrollTop {
		return true
	}
	n := max(int(params.Params[0]), 1)
	for y := buf.ScrollTop; y <= buf.ScrollBottom; y++ {
		line := buf.Lines.Get(buf.YBase + y)
		if line != nil {
			line.DeleteCells(0, n, buf.GetNullCell(h.eraseAttrData()))
			line.IsWrapped = false
		}
	}
	h.dirtyRowTracker.MarkRangeDirty(buf.ScrollTop, buf.ScrollBottom)
	return true
}

func (h *InputHandler) scrollRight(params *Params) bool {
	buf := h.activeBuffer()
	if buf.Y > buf.ScrollBottom || buf.Y < buf.ScrollTop {
		return true
	}
	n := max(int(params.Params[0]), 1)
	for y := buf.ScrollTop; y <= buf.ScrollBottom; y++ {
		line := buf.Lines.Get(buf.YBase + y)
		if line != nil {
			line.InsertCells(0, n, buf.GetNullCell(h.eraseAttrData()))
			line.IsWrapped = false
		}
	}
	h.dirtyRowTracker.MarkRangeDirty(buf.ScrollTop, buf.ScrollBottom)
	return true
}

func (h *InputHandler) insertColumns(params *Params) bool {
	buf := h.activeBuffer()
	if buf.Y > buf.ScrollBottom || buf.Y < buf.ScrollTop {
		return true
	}
	n := max(int(params.Params[0]), 1)
	for y := buf.ScrollTop; y <= buf.ScrollBottom; y++ {
		line := buf.Lines.Get(buf.YBase + y)
		if line != nil {
			line.InsertCells(buf.X, n, buf.GetNullCell(h.eraseAttrData()))
			line.IsWrapped = false
		}
	}
	h.dirtyRowTracker.MarkRangeDirty(buf.ScrollTop, buf.ScrollBottom)
	return true
}

func (h *InputHandler) deleteColumns(params *Params) bool {
	buf := h.activeBuffer()
	if buf.Y > buf.ScrollBottom || buf.Y < buf.ScrollTop {
		return true
	}
	n := max(int(params.Params[0]), 1)
	for y := buf.ScrollTop; y <= buf.ScrollBottom; y++ {
		line := buf.Lines.Get(buf.YBase + y)
		if line != nil {
			line.DeleteCells(buf.X, n, buf.GetNullCell(h.eraseAttrData()))
			line.IsWrapped = false
		}
	}
	h.dirtyRowTracker.MarkRangeDirty(buf.ScrollTop, buf.ScrollBottom)
	return true
}

// --- Repeat ---

func (h *InputHandler) repeatPrecedingCharacter(params *Params) bool {
	joinState := h.parser.PrecedingJoinState()
	if joinState == 0 {
		return true
	}
	n := max(int(params.Params[0]), 1)
	buf := h.activeBuffer()
	chWidth := ExtractCharPropsWidth(joinState)
	x := buf.X - chWidth
	if x < 0 {
		x = 0
	}
	bufferRow := buf.Lines.Get(buf.YBase + buf.Y)
	if bufferRow == nil {
		return true
	}
	text := bufferRow.GetString(x)
	data := make([]uint32, 0, len(text)*n)
	for _, r := range text {
		data = append(data, uint32(r))
	}
	idata := len(data)
	for i := 1; i < n; i++ {
		data = append(data, data[:idata]...)
	}
	h.Print(data, 0, len(data))
	return true
}

// --- Device attributes ---

func (h *InputHandler) sendDeviceAttributesPrimary(params *Params) bool {
	if params.Params[0] > 0 {
		return true
	}
	h.coreService.TriggerDataEvent("\x1b[?1;2c", false, false)
	return true
}

func (h *InputHandler) sendDeviceAttributesSecondary(params *Params) bool {
	if params.Params[0] > 0 {
		return true
	}
	h.coreService.TriggerDataEvent("\x1b[>0;276;0c", false, false)
	return true
}

// sendXtVersion responds to XTVERSION (CSI > q) with a DCS response.
func (h *InputHandler) sendXtVersion(params *Params) bool {
	if params.Params[0] > 0 {
		return true
	}
	h.coreService.TriggerDataEvent("\x1bP>|xterm-go(0.1.0)\x1b\\", false, false)
	return true
}

func (h *InputHandler) deviceStatus(params *Params) bool {
	buf := h.activeBuffer()
	switch params.Params[0] {
	case 5:
		h.coreService.TriggerDataEvent("\x1b[0n", false, false)
	case 6:
		y := buf.Y + 1
		x := buf.X + 1
		h.coreService.TriggerDataEvent(fmt.Sprintf("\x1b[%d;%dR", y, x), false, false)
	}
	return true
}

func (h *InputHandler) deviceStatusPrivate(params *Params) bool {
	buf := h.activeBuffer()
	if params.Params[0] == 6 {
		y := buf.Y + 1
		x := buf.X + 1
		h.coreService.TriggerDataEvent(fmt.Sprintf("\x1b[?%d;%dR", y, x), false, false)
	}
	return true
}

// --- Soft reset ---

func (h *InputHandler) softReset(_ *Params) bool {
	h.coreService.IsCursorHidden = false
	h.OnRequestSyncScrollBarEmitter.Fire(struct{}{})
	buf := h.activeBuffer()
	buf.ScrollTop = 0
	buf.ScrollBottom = h.bufferService.Rows - 1
	h.curAttrData = DefaultAttrData()
	h.coreService.Reset()
	h.charsetService.Reset()
	buf.SavedState.X = 0
	buf.SavedState.Y = buf.YBase
	buf.SavedState.CurAttrData.Fg = h.curAttrData.Fg
	buf.SavedState.CurAttrData.Bg = h.curAttrData.Bg
	buf.SavedState.Charset = h.charsetService.Charset
	h.coreService.DecPrivateModes.Origin = false
	return true
}

// --- Cursor style ---

func (h *InputHandler) setCursorStyle(params *Params) bool {
	p := int32(1)
	if params.Length > 0 {
		p = params.Params[0]
	}
	if p == 0 {
		h.coreService.DecPrivateModes.CursorStyle = nil
		h.coreService.DecPrivateModes.CursorBlinkOverride = nil
	} else {
		var style CursorStyle
		switch p {
		case 1, 2:
			style = CursorStyleBlock
		case 3, 4:
			style = CursorStyleUnderline
		case 5, 6:
			style = CursorStyleBar
		default:
			style = CursorStyleBlock
		}
		h.coreService.DecPrivateModes.CursorStyle = &style
		isBlinking := p%2 == 1
		h.coreService.DecPrivateModes.CursorBlinkOverride = &isBlinking
	}
	return true
}

// --- Scroll region ---

func (h *InputHandler) setScrollRegion(params *Params) bool {
	top := max(int(params.Params[0]), 1)
	bottom := h.bufferService.Rows
	if params.Length >= 2 && params.Params[1] > 0 && int(params.Params[1]) <= h.bufferService.Rows {
		bottom = int(params.Params[1])
	}
	if bottom > top {
		buf := h.activeBuffer()
		buf.ScrollTop = top - 1
		buf.ScrollBottom = bottom - 1
		h.setCursor(0, 0)
	}
	return true
}

// --- Save/Restore cursor (CSI s / CSI u) ---

func (h *InputHandler) csiSaveCursor(_ *Params) bool {
	return h.SaveCursor()
}

func (h *InputHandler) csiRestoreCursor(_ *Params) bool {
	return h.RestoreCursor()
}

// --- Select character protection ---

func (h *InputHandler) selectProtected(params *Params) bool {
	p := params.Params[0]
	if p == 1 {
		h.curAttrData.Bg |= BgFlagProtected
	}
	if p == 2 || p == 0 {
		h.curAttrData.Bg &^= BgFlagProtected
	}
	return true
}

// --- Mode set/reset ---

func (h *InputHandler) setMode(params *Params) bool {
	for i := range params.Length {
		switch params.Params[i] {
		case 4:
			h.coreService.Modes.InsertMode = true
		case 20:
			h.optionsService.Options.ConvertEol = true
		}
	}
	return true
}

func (h *InputHandler) resetMode(params *Params) bool {
	for i := range params.Length {
		switch params.Params[i] {
		case 4:
			h.coreService.Modes.InsertMode = false
		case 20:
			h.optionsService.Options.ConvertEol = false
		}
	}
	return true
}

func (h *InputHandler) setModePrivate(params *Params) bool {
	for i := range params.Length {
		switch params.Params[i] {
		case 1:
			h.coreService.DecPrivateModes.ApplicationCursorKeys = true
		case 6:
			h.coreService.DecPrivateModes.Origin = true
			h.setCursor(0, 0)
		case 7:
			h.coreService.DecPrivateModes.Wraparound = true
		case 25:
			h.coreService.IsCursorHidden = false
		case 45:
			h.coreService.DecPrivateModes.ReverseWraparound = true
		case 66:
			h.coreService.DecPrivateModes.ApplicationKeypad = true
			h.OnRequestSyncScrollBarEmitter.Fire(struct{}{})
		case 9:
			h.coreService.DecPrivateModes.MouseTrackingMode = "X10"
		case 1000:
			h.coreService.DecPrivateModes.MouseTrackingMode = "VT200"
		case 1002:
			h.coreService.DecPrivateModes.MouseTrackingMode = "DRAG"
		case 1003:
			h.coreService.DecPrivateModes.MouseTrackingMode = "ANY"
		case 1004:
			h.coreService.DecPrivateModes.SendFocus = true
		case 1006:
			h.coreService.DecPrivateModes.MouseEncoding = "SGR"
		case 1016:
			h.coreService.DecPrivateModes.MouseEncoding = "SGR_PIXELS"
		case 1048:
			h.SaveCursor()
		case 1049:
			h.SaveCursor()
			fallthrough
		case 47, 1047:
			// Swap kitty keyboard flags: save main, restore alt
			kk := &h.coreService.KittyKeyboard
			kk.MainFlags = kk.Flags
			kk.Flags = kk.AltFlags
			kk.MainStack, kk.AltStack = kk.AltStack, kk.MainStack
			h.bufferService.Buffers.ActivateAltBuffer(h.eraseAttrData())
			h.coreService.IsCursorInitialized = true
			h.OnRequestRefreshRowsEmitter.Fire(RowRange{})
			h.OnRequestSyncScrollBarEmitter.Fire(struct{}{})
		case 2004:
			h.coreService.DecPrivateModes.BracketedPasteMode = true
		case 2026:
			h.coreService.DecPrivateModes.SynchronizedOutput = true
		case 2031:
			h.coreService.DecPrivateModes.ColorSchemeUpdates = true
		case 9001:
			h.coreService.DecPrivateModes.Win32InputMode = true
		}
	}
	return true
}

func (h *InputHandler) resetModePrivate(params *Params) bool {
	for i := range params.Length {
		switch params.Params[i] {
		case 1:
			h.coreService.DecPrivateModes.ApplicationCursorKeys = false
		case 6:
			h.coreService.DecPrivateModes.Origin = false
			h.setCursor(0, 0)
		case 7:
			h.coreService.DecPrivateModes.Wraparound = false
		case 25:
			h.coreService.IsCursorHidden = true
		case 45:
			h.coreService.DecPrivateModes.ReverseWraparound = false
		case 66:
			h.coreService.DecPrivateModes.ApplicationKeypad = false
			h.OnRequestSyncScrollBarEmitter.Fire(struct{}{})
		case 9, 1000, 1002, 1003:
			h.coreService.DecPrivateModes.MouseTrackingMode = "NONE"
		case 1004:
			h.coreService.DecPrivateModes.SendFocus = false
		case 1006:
			h.coreService.DecPrivateModes.MouseEncoding = "DEFAULT"
		case 1016:
			h.coreService.DecPrivateModes.MouseEncoding = "DEFAULT"
		case 1048:
			h.RestoreCursor()
		case 1049:
			// Swap kitty keyboard flags: save alt, restore main
			kk := &h.coreService.KittyKeyboard
			kk.AltFlags = kk.Flags
			kk.Flags = kk.MainFlags
			kk.MainStack, kk.AltStack = kk.AltStack, kk.MainStack
			h.bufferService.Buffers.ActivateNormalBuffer()
			h.RestoreCursor()
			h.coreService.IsCursorInitialized = true
			h.OnRequestRefreshRowsEmitter.Fire(RowRange{})
			h.OnRequestSyncScrollBarEmitter.Fire(struct{}{})
		case 47, 1047:
			// Swap kitty keyboard flags: save alt, restore main
			kk := &h.coreService.KittyKeyboard
			kk.AltFlags = kk.Flags
			kk.Flags = kk.MainFlags
			kk.MainStack, kk.AltStack = kk.AltStack, kk.MainStack
			h.bufferService.Buffers.ActivateNormalBuffer()
			h.coreService.IsCursorInitialized = true
			h.OnRequestRefreshRowsEmitter.Fire(RowRange{})
			h.OnRequestSyncScrollBarEmitter.Fire(struct{}{})
		case 2004:
			h.coreService.DecPrivateModes.BracketedPasteMode = false
		case 2026:
			h.coreService.DecPrivateModes.SynchronizedOutput = false
			h.OnRequestRefreshRowsEmitter.Fire(RowRange{})
		case 2031:
			h.coreService.DecPrivateModes.ColorSchemeUpdates = false
		case 9001:
			h.coreService.DecPrivateModes.Win32InputMode = false
		}
	}
	return true
}
