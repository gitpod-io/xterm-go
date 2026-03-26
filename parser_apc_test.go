package xterm

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// testApcHandler records calls for testing.
type testApcHandler struct {
	starts int
	puts   []string
	ends   []bool
	endRet bool
}

func (h *testApcHandler) Start() { h.starts++ }
func (h *testApcHandler) Put(data []uint32, start, end int) {
	h.puts = append(h.puts, utf32ToString(data, start, end))
}
func (h *testApcHandler) End(success bool) bool { h.ends = append(h.ends, success); return h.endRet }

func TestApcParserStartPutEnd(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Starts int
		Puts   []string
		Ends   []bool
	}
	type TestCase struct {
		Name     string
		Ident    int
		Input    string
		Success  bool
		Expected Expectation
	}
	tests := []TestCase{
		{
			Name:    "basic lifecycle with char ident",
			Ident:   'G',
			Input:   "Gf=100,a=T;base64data",
			Success: true,
			Expected: Expectation{
				Starts: 1,
				Puts:   []string{"f=100,a=T;base64data"},
				Ends:   []bool{true},
			},
		},
		{
			Name:    "ident only no payload",
			Ident:   'X',
			Input:   "X",
			Success: true,
			Expected: Expectation{
				Starts: 1,
				Puts:   nil,
				Ends:   []bool{true},
			},
		},
		{
			Name:    "unsuccessful end",
			Ident:   'G',
			Input:   "Gdata",
			Success: false,
			Expected: Expectation{
				Starts: 1,
				Puts:   []string{"data"},
				Ends:   []bool{false},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			p := NewApcParser()
			h := &testApcHandler{endRet: true}
			p.RegisterHandler(tc.Ident, h)

			data := toUint32(tc.Input)
			p.Start()
			p.Put(data, 0, len(data))
			p.End(tc.Success)

			got := Expectation{Starts: h.starts, Puts: h.puts, Ends: h.ends}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestApcParserHandlerDispatch(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		H1Starts int
		H2Starts int
		H1Ends   int
		H2Ends   int
	}

	p := NewApcParser()
	h1 := &testApcHandler{}
	h2 := &testApcHandler{}
	p.RegisterHandler('G', h1)
	p.RegisterHandler('G', h2)

	data := toUint32("Gpayload")
	p.Start()
	p.Put(data, 0, len(data))
	p.End(true)

	got := Expectation{
		H1Starts: h1.starts,
		H2Starts: h2.starts,
		H1Ends:   len(h1.ends),
		H2Ends:   len(h2.ends),
	}
	expected := Expectation{H1Starts: 1, H2Starts: 1, H1Ends: 1, H2Ends: 1}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestApcParserFallback(t *testing.T) {
	t.Parallel()
	type FallbackCall struct {
		Ident  int
		Action string
	}
	type Expectation struct {
		Calls []FallbackCall
	}

	p := NewApcParser()
	var calls []FallbackCall
	p.SetHandlerFallback(func(ident int, action string, payload ...interface{}) {
		calls = append(calls, FallbackCall{Ident: ident, Action: action})
	})

	data := toUint32("Zpayload")
	p.Start()
	p.Put(data, 0, len(data))
	p.End(true)

	got := Expectation{Calls: calls}
	expected := Expectation{Calls: []FallbackCall{
		{Ident: 'Z', Action: "START"},
		{Ident: 'Z', Action: "PUT"},
		{Ident: 'Z', Action: "END"},
	}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestApcParserDispose(t *testing.T) {
	t.Parallel()
	p := NewApcParser()
	h := &testApcHandler{}
	d := p.RegisterHandler('G', h)
	d.Dispose()

	data := toUint32("Gtest")
	p.Start()
	p.Put(data, 0, len(data))
	p.End(true)

	if h.starts != 0 {
		t.Errorf("handler should not have been called after dispose, got %d starts", h.starts)
	}
}

func TestApcParserReset(t *testing.T) {
	t.Parallel()
	p := NewApcParser()
	h := &testApcHandler{}
	p.RegisterHandler('G', h)

	data := toUint32("Gpartial")
	p.Start()
	p.Put(data, 0, len(data))
	p.Reset()

	type Expectation struct {
		Ends []bool
	}
	got := Expectation{Ends: h.ends}
	expected := Expectation{Ends: []bool{false}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestApcParserEmptySequence(t *testing.T) {
	t.Parallel()
	// Empty APC (end in ID state) should just reset without calling handlers
	p := NewApcParser()
	h := &testApcHandler{}
	p.RegisterHandler('G', h)

	p.Start()
	p.End(true)

	if h.starts != 0 {
		t.Errorf("handler should not start on empty APC, got %d starts", h.starts)
	}
}

func TestApcParserClearHandler(t *testing.T) {
	t.Parallel()
	p := NewApcParser()
	h := &testApcHandler{}
	p.RegisterHandler('G', h)
	p.ClearHandler('G')

	data := toUint32("Gtest")
	p.Start()
	p.Put(data, 0, len(data))
	p.End(true)

	if h.starts != 0 {
		t.Errorf("handler should not be called after ClearHandler, got %d starts", h.starts)
	}
}

func TestApcParserEndConsumption(t *testing.T) {
	t.Parallel()
	p := NewApcParser()
	h1 := &testApcHandler{endRet: false}
	h2 := &testApcHandler{endRet: true}
	p.RegisterHandler('G', h1)
	p.RegisterHandler('G', h2)

	data := toUint32("Gdata")
	p.Start()
	p.Put(data, 0, len(data))
	p.End(true)

	type Expectation struct {
		H1Ends []bool
		H2Ends []bool
	}
	got := Expectation{H1Ends: h1.ends, H2Ends: h2.ends}
	expected := Expectation{H1Ends: []bool{false}, H2Ends: []bool{true}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

func TestApcStringHandler(t *testing.T) {
	t.Parallel()
	type Expectation struct {
		Received string
		Result   bool
	}
	type TestCase struct {
		Name     string
		Payload  string
		Success  bool
		Expected Expectation
	}
	tests := []TestCase{
		{
			Name:     "successful end",
			Payload:  "hello world",
			Success:  true,
			Expected: Expectation{Received: "hello world", Result: true},
		},
		{
			Name:     "unsuccessful end",
			Payload:  "hello",
			Success:  false,
			Expected: Expectation{Received: "", Result: false},
		},
	}
	for _, tc := range tests {
		t.Run(tc.Name, func(t *testing.T) {
			t.Parallel()
			var received string
			h := NewApcStringHandler(func(data string) bool {
				received = data
				return true
			})
			h.Start()
			data := toUint32(tc.Payload)
			h.Put(data, 0, len(data))
			result := h.End(tc.Success)

			got := Expectation{Received: received, Result: result}
			if diff := cmp.Diff(tc.Expected, got); diff != "" {
				t.Errorf("(-want +got):\n%s", diff)
			}
		})
	}
}

func TestApcStringHandlerPayloadLimit(t *testing.T) {
	t.Parallel()
	var received string
	h := NewApcStringHandler(func(data string) bool {
		received = data
		return true
	})
	h.Start()
	big := strings.Repeat("C", ParserPayloadLimit+1)
	data := toUint32(big)
	h.Put(data, 0, len(data))
	result := h.End(true)

	if result != false {
		t.Error("expected false when payload limit exceeded")
	}
	if received != "" {
		t.Error("handler should not have been called when limit exceeded")
	}
}

func TestApcParserMultiplePuts(t *testing.T) {
	t.Parallel()
	p := NewApcParser()
	h := &testApcHandler{endRet: true}
	p.RegisterHandler('G', h)

	// Send ident and payload in separate Put calls
	id := toUint32("G")
	p.Start()
	p.Put(id, 0, len(id))

	p1 := toUint32("hel")
	p.Put(p1, 0, len(p1))

	p2 := toUint32("lo")
	p.Put(p2, 0, len(p2))

	p.End(true)

	type Expectation struct {
		Starts int
		Puts   []string
	}
	got := Expectation{Starts: h.starts, Puts: h.puts}
	expected := Expectation{Starts: 1, Puts: []string{"hel", "lo"}}
	if diff := cmp.Diff(expected, got); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}
