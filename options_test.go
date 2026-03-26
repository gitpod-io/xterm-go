package xterm

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDefaultOptions(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Cols         int
		Rows         int
		Scrollback   int
		CursorStyle  CursorStyle
		CursorWidth  int
		TabStopWidth int
		FontFamily   string
		FontSize     int
		LineHeight   float64
		LogLevel     LogLevel
		TermName     string
	}

	opts := DefaultOptions()
	got := Expectation{
		Cols:         opts.Cols,
		Rows:         opts.Rows,
		Scrollback:   opts.Scrollback,
		CursorStyle:  opts.CursorStyle,
		CursorWidth:  opts.CursorWidth,
		TabStopWidth: opts.TabStopWidth,
		FontFamily:   opts.FontFamily,
		FontSize:     opts.FontSize,
		LineHeight:   opts.LineHeight,
		LogLevel:     opts.LogLevel,
		TermName:     opts.TermName,
	}
	expected := Expectation{
		Cols:         80,
		Rows:         24,
		Scrollback:   1000,
		CursorStyle:  CursorStyleBlock,
		CursorWidth:  1,
		TabStopWidth: 8,
		FontFamily:   "monospace",
		FontSize:     15,
		LineHeight:   1.0,
		LogLevel:     LogLevelInfo,
		TermName:     "xterm",
	}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestNewOptionsServiceDefaults(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Cols       int
		Rows       int
		Scrollback int
	}

	s := NewOptionsService(nil)
	got := Expectation{
		Cols:       s.Options.Cols,
		Rows:       s.Options.Rows,
		Scrollback: s.Options.Scrollback,
	}
	expected := Expectation{Cols: 80, Rows: 24, Scrollback: 1000}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestNewOptionsServiceOverrides(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Cols       int
		Rows       int
		Scrollback int
		FontFamily string
	}

	s := NewOptionsService(&TerminalOptions{
		Cols:       120,
		Rows:       40,
		Scrollback: 5000,
	})
	got := Expectation{
		Cols:       s.Options.Cols,
		Rows:       s.Options.Rows,
		Scrollback: s.Options.Scrollback,
		FontFamily: s.Options.FontFamily,
	}
	expected := Expectation{Cols: 120, Rows: 40, Scrollback: 5000, FontFamily: "monospace"}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestOptionsServiceSetOption(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		Cols       int
		FiredEvent string
	}

	s := NewOptionsService(nil)
	var firedName string
	s.OnOptionChangeEmitter.Event(func(name string) {
		firedName = name
	})
	s.SetOption("cols", 120)

	got := Expectation{Cols: s.Options.Cols, FiredEvent: firedName}
	expected := Expectation{Cols: 120, FiredEvent: "cols"}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestOptionsServiceSetOptionNoChangeNoFire(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		FireCount int
	}

	s := NewOptionsService(nil)
	count := 0
	s.OnOptionChangeEmitter.Event(func(string) { count++ })
	s.SetOption("cols", 80) // same as default

	got := Expectation{FireCount: count}
	expected := Expectation{FireCount: 0}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestOptionsServiceSetOptionScrollback(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		Name     string
		Value    int
		Expected int
	}
	tests := []TestCase{
		{"normal value", 500, 500},
		{"negative clamped to 0", -1, 0},
		{"max clamped", MaxBufferSize + 1, MaxBufferSize},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			s := NewOptionsService(nil)
			s.SetOption("scrollback", tc.Value)

			type Expectation struct {
				Scrollback int
			}
			got := Expectation{Scrollback: s.Options.Scrollback}
			expected := Expectation{Scrollback: tc.Expected}
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestOptionsServiceSetOptionCursorStyle(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		Name     string
		Value    CursorStyle
		Expected CursorStyle
	}
	tests := []TestCase{
		{"block", CursorStyleBlock, CursorStyleBlock},
		{"underline", CursorStyleUnderline, CursorStyleUnderline},
		{"bar", CursorStyleBar, CursorStyleBar},
		{"invalid stays default", CursorStyle("invalid"), CursorStyleBlock},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			s := NewOptionsService(nil)
			s.SetOption("cursorStyle", tc.Value)

			type Expectation struct {
				CursorStyle CursorStyle
			}
			got := Expectation{CursorStyle: s.Options.CursorStyle}
			expected := Expectation{CursorStyle: tc.Expected}
			if diff := cmp.Diff(expected, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestOptionsServiceSetOptionTabStopWidth(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		TabStopWidth int
	}

	t.Run("valid value", func(t *testing.T) {
		t.Parallel()
		s := NewOptionsService(nil)
		s.SetOption("tabStopWidth", 4)
		got := Expectation{TabStopWidth: s.Options.TabStopWidth}
		expected := Expectation{TabStopWidth: 4}
		if diff := cmp.Diff(expected, got); diff != "" {
			t.Errorf("(-want +got):\n%s", diff)
		}
	})

	t.Run("zero rejected", func(t *testing.T) {
		t.Parallel()
		s := NewOptionsService(nil)
		s.SetOption("tabStopWidth", 0)
		got := Expectation{TabStopWidth: s.Options.TabStopWidth}
		expected := Expectation{TabStopWidth: 8} // default unchanged
		if diff := cmp.Diff(expected, got); diff != "" {
			t.Errorf("(-want +got):\n%s", diff)
		}
	})
}

func TestOptionsServiceOnSpecificOptionChange(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		CallCount int
	}

	s := NewOptionsService(nil)
	count := 0
	s.OnSpecificOptionChange("cols", func() { count++ })
	s.SetOption("cols", 120)
	s.SetOption("rows", 40) // should not trigger

	got := Expectation{CallCount: count}
	expected := Expectation{CallCount: 1}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestOptionsServiceOnMultipleOptionChange(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		CallCount int
	}

	s := NewOptionsService(nil)
	count := 0
	s.OnMultipleOptionChange([]string{"cols", "rows"}, func() { count++ })
	s.SetOption("cols", 120)
	s.SetOption("rows", 40)
	s.SetOption("cursorBlink", true) // should not trigger

	got := Expectation{CallCount: count}
	expected := Expectation{CallCount: 2}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestOptionsServiceDispose(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		CallCount int
	}

	s := NewOptionsService(nil)
	count := 0
	s.OnOptionChangeEmitter.Event(func(string) { count++ })
	s.Dispose()
	s.SetOption("cols", 120)

	got := Expectation{CallCount: count}
	expected := Expectation{CallCount: 0}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestOptionsServiceSetOptionConvertEol(t *testing.T) {
	t.Parallel()

	type Expectation struct {
		ConvertEol bool
		FireCount  int
	}

	s := NewOptionsService(nil)
	count := 0
	s.OnOptionChangeEmitter.Event(func(string) { count++ })
	s.SetOption("convertEol", true)

	got := Expectation{ConvertEol: s.Options.ConvertEol, FireCount: count}
	expected := Expectation{ConvertEol: true, FireCount: 1}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}
