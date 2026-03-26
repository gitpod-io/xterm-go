package xterm

// Ported from xterm.js src/common/services/OptionsService.ts.

// LogLevel controls terminal logging verbosity.
type LogLevel string

const (
	LogLevelTrace   LogLevel = "trace"
	LogLevelDebug   LogLevel = "debug"
	LogLevelInfo    LogLevel = "info"
	LogLevelWarning LogLevel = "warning"
	LogLevelError   LogLevel = "error"
	LogLevelOff     LogLevel = "off"
)

// FontWeight represents CSS font-weight values.
type FontWeight string

const (
	FontWeightNormal FontWeight = "normal"
	FontWeightBold   FontWeight = "bold"
	FontWeight100    FontWeight = "100"
	FontWeight200    FontWeight = "200"
	FontWeight300    FontWeight = "300"
	FontWeight400    FontWeight = "400"
	FontWeight500    FontWeight = "500"
	FontWeight600    FontWeight = "600"
	FontWeight700    FontWeight = "700"
	FontWeight800    FontWeight = "800"
	FontWeight900    FontWeight = "900"
)

// WindowsPty holds Windows pseudo-terminal backend configuration.
type WindowsPty struct {
	Backend string `json:"backend,omitempty"`
	BuildNo int    `json:"buildNumber,omitempty"`
}

// ScrollbarOptions configures the scrollbar.
type ScrollbarOptions struct {
	ShowScrollbar bool `json:"showScrollbar"`
}

// TerminalOptions holds all terminal configuration fields.
type TerminalOptions struct {
	Cols                          int                 `json:"cols"`
	Rows                          int                 `json:"rows"`
	ShowCursorImmediately         bool                `json:"showCursorImmediately"`
	CursorBlink                   bool                `json:"cursorBlink"`
	BlinkIntervalDuration         int                 `json:"blinkIntervalDuration"`
	CursorStyle                   CursorStyle         `json:"cursorStyle"`
	CursorWidth                   int                 `json:"cursorWidth"`
	CursorInactiveStyle           CursorInactiveStyle `json:"cursorInactiveStyle"`
	DrawBoldTextInBrightColors    bool                `json:"drawBoldTextInBrightColors"`
	FastScrollSensitivity         float64             `json:"fastScrollSensitivity"`
	FontFamily                    string              `json:"fontFamily"`
	FontSize                      int                 `json:"fontSize"`
	FontWeight                    FontWeight          `json:"fontWeight"`
	FontWeightBold                FontWeight          `json:"fontWeightBold"`
	IgnoreBracketedPasteMode      bool                `json:"ignoreBracketedPasteMode"`
	LineHeight                    float64             `json:"lineHeight"`
	LetterSpacing                 float64             `json:"letterSpacing"`
	LogLevel                      LogLevel            `json:"logLevel"`
	Scrollback                    int                 `json:"scrollback"`
	Scrollbar                     ScrollbarOptions    `json:"scrollbar"`
	ScrollOnEraseInDisplay        bool                `json:"scrollOnEraseInDisplay"`
	ScrollOnUserInput             bool                `json:"scrollOnUserInput"`
	ScrollSensitivity             float64             `json:"scrollSensitivity"`
	ScreenReaderMode              bool                `json:"screenReaderMode"`
	SmoothScrollDuration          int                 `json:"smoothScrollDuration"`
	MacOptionIsMeta               bool                `json:"macOptionIsMeta"`
	MacOptionClickForcesSelection bool                `json:"macOptionClickForcesSelection"`
	MinimumContrastRatio          float64             `json:"minimumContrastRatio"`
	DisableStdin                  bool                `json:"disableStdin"`
	AllowProposedApi              bool                `json:"allowProposedApi"`
	AllowTransparency             bool                `json:"allowTransparency"`
	TabStopWidth                  int                 `json:"tabStopWidth"`
	ReflowCursorLine              bool                `json:"reflowCursorLine"`
	RescaleOverlappingGlyphs      bool                `json:"rescaleOverlappingGlyphs"`
	RightClickSelectsWord         bool                `json:"rightClickSelectsWord"`
	WordSeparator                 string              `json:"wordSeparator"`
	AltClickMovesCursor           bool                `json:"altClickMovesCursor"`
	ConvertEol                    bool                `json:"convertEol"`
	TermName                      string              `json:"termName"`
	WindowsPty                    WindowsPty          `json:"windowsPty"`
}

// DefaultOptions returns TerminalOptions with sensible defaults matching xterm.js.
func DefaultOptions() TerminalOptions {
	return TerminalOptions{
		Cols:                       80,
		Rows:                       24,
		CursorBlink:                false,
		CursorStyle:                CursorStyleBlock,
		CursorWidth:                1,
		CursorInactiveStyle:        CursorInactiveStyleOutline,
		DrawBoldTextInBrightColors: true,
		FastScrollSensitivity:      5,
		FontFamily:                 "monospace",
		FontSize:                   15,
		FontWeight:                 FontWeightNormal,
		FontWeightBold:             FontWeightBold,
		LineHeight:                 1.0,
		LogLevel:                   LogLevelInfo,
		Scrollback:                 1000,
		Scrollbar:                  ScrollbarOptions{ShowScrollbar: true},
		ScrollOnUserInput:          true,
		ScrollSensitivity:          1,
		MinimumContrastRatio:       1,
		TabStopWidth:               8,
		WordSeparator:              " ()[]{}',\"`\\",
		AltClickMovesCursor:        true,
		TermName:                   "xterm",
	}
}

// OptionsService wraps TerminalOptions with change notification.
type OptionsService struct {
	Options TerminalOptions

	OnOptionChangeEmitter EventEmitter[string]
}

// NewOptionsService creates an OptionsService with defaults merged with the given overrides.
func NewOptionsService(opts *TerminalOptions) *OptionsService {
	s := &OptionsService{
		Options: DefaultOptions(),
	}
	if opts != nil {
		s.applyOverrides(opts)
	}
	return s
}

// applyOverrides merges non-zero override values into the current options.
// Only fields explicitly set (non-zero) in the override are applied.
func (s *OptionsService) applyOverrides(opts *TerminalOptions) {
	if opts.Cols != 0 {
		s.Options.Cols = opts.Cols
	}
	if opts.Rows != 0 {
		s.Options.Rows = opts.Rows
	}
	if opts.CursorBlink {
		s.Options.CursorBlink = opts.CursorBlink
	}
	if opts.CursorStyle != "" {
		s.Options.CursorStyle = opts.CursorStyle
	}
	if opts.CursorWidth != 0 {
		s.Options.CursorWidth = opts.CursorWidth
	}
	if opts.CursorInactiveStyle != "" {
		s.Options.CursorInactiveStyle = opts.CursorInactiveStyle
	}
	if opts.FontFamily != "" {
		s.Options.FontFamily = opts.FontFamily
	}
	if opts.FontSize != 0 {
		s.Options.FontSize = opts.FontSize
	}
	if opts.FontWeight != "" {
		s.Options.FontWeight = opts.FontWeight
	}
	if opts.FontWeightBold != "" {
		s.Options.FontWeightBold = opts.FontWeightBold
	}
	if opts.LineHeight != 0 {
		s.Options.LineHeight = opts.LineHeight
	}
	if opts.LetterSpacing != 0 {
		s.Options.LetterSpacing = opts.LetterSpacing
	}
	if opts.LogLevel != "" {
		s.Options.LogLevel = opts.LogLevel
	}
	if opts.Scrollback != 0 {
		s.Options.Scrollback = opts.Scrollback
	}
	if opts.ScrollSensitivity != 0 {
		s.Options.ScrollSensitivity = opts.ScrollSensitivity
	}
	if opts.FastScrollSensitivity != 0 {
		s.Options.FastScrollSensitivity = opts.FastScrollSensitivity
	}
	if opts.TabStopWidth != 0 {
		s.Options.TabStopWidth = opts.TabStopWidth
	}
	if opts.WordSeparator != "" {
		s.Options.WordSeparator = opts.WordSeparator
	}
	if opts.TermName != "" {
		s.Options.TermName = opts.TermName
	}
	if opts.MinimumContrastRatio != 0 {
		s.Options.MinimumContrastRatio = opts.MinimumContrastRatio
	}
	s.Options.ConvertEol = opts.ConvertEol
	s.Options.DisableStdin = opts.DisableStdin
	s.Options.ScrollOnUserInput = opts.ScrollOnUserInput
	s.Options.ShowCursorImmediately = opts.ShowCursorImmediately
	s.Options.AllowTransparency = opts.AllowTransparency
	s.Options.AllowProposedApi = opts.AllowProposedApi
	s.Options.ScreenReaderMode = opts.ScreenReaderMode
	s.Options.MacOptionIsMeta = opts.MacOptionIsMeta
	s.Options.AltClickMovesCursor = opts.AltClickMovesCursor
	s.Options.WindowsPty = opts.WindowsPty
}

// SetOption sets a named option and fires the change event if the value changed.
func (s *OptionsService) SetOption(name string, value interface{}) {
	changed := false
	switch name {
	case "cols":
		if v, ok := value.(int); ok && v != s.Options.Cols {
			s.Options.Cols = v
			changed = true
		}
	case "rows":
		if v, ok := value.(int); ok && v != s.Options.Rows {
			s.Options.Rows = v
			changed = true
		}
	case "scrollback":
		if v, ok := value.(int); ok && v != s.Options.Scrollback {
			if v < 0 {
				v = 0
			}
			if v > MaxBufferSize {
				v = MaxBufferSize
			}
			s.Options.Scrollback = v
			changed = true
		}
	case "tabStopWidth":
		if v, ok := value.(int); ok && v != s.Options.TabStopWidth {
			if v >= 1 {
				s.Options.TabStopWidth = v
				changed = true
			}
		}
	case "cursorStyle":
		if v, ok := value.(CursorStyle); ok && v != s.Options.CursorStyle {
			if v == CursorStyleBlock || v == CursorStyleUnderline || v == CursorStyleBar {
				s.Options.CursorStyle = v
				changed = true
			}
		}
	case "cursorBlink":
		if v, ok := value.(bool); ok && v != s.Options.CursorBlink {
			s.Options.CursorBlink = v
			changed = true
		}
	case "convertEol":
		if v, ok := value.(bool); ok && v != s.Options.ConvertEol {
			s.Options.ConvertEol = v
			changed = true
		}
	default:
		// Unknown options are silently ignored.
		return
	}
	if changed {
		s.OnOptionChangeEmitter.Fire(name)
	}
}

// OnSpecificOptionChange registers a listener for changes to a specific option.
func (s *OptionsService) OnSpecificOptionChange(key string, listener func()) Disposable {
	return s.OnOptionChangeEmitter.Event(func(name string) {
		if name == key {
			listener()
		}
	})
}

// OnMultipleOptionChange registers a listener that fires when any of the given options change.
func (s *OptionsService) OnMultipleOptionChange(keys []string, listener func()) Disposable {
	keySet := make(map[string]bool, len(keys))
	for _, k := range keys {
		keySet[k] = true
	}
	return s.OnOptionChangeEmitter.Event(func(name string) {
		if keySet[name] {
			listener()
		}
	})
}

// Dispose cleans up the options service.
func (s *OptionsService) Dispose() {
	s.OnOptionChangeEmitter.Dispose()
}
