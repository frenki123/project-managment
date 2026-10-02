package drops

import "errors"

func f() (int, error)                  { return 1, errors.New("x") }
func g() (int, error)                  { return 1, errors.New("y") }
func parseInt(s string) (int64, error) { return 1, nil }
func readAll() ([]byte, error)         { return nil, nil }
func multi(a, b int) (int, error)      { return a + b, errors.New("m") }

func droppedErrorShortVar() {
	x, _ := f() // want `assigned value is dropped \(1 blank identifier\)`
	_ = x
}

func droppedNonErrorValue() {
	_, err := f() // want `assigned value is dropped \(1 blank identifier\)`
	if err != nil {
		return
	}
}

func droppedBoth() {
	_, _ = f() // want `assigned value is dropped \(2 blank identifiers\)`
}

func droppedSingleAssign() {
	var x int
	x, _ = f() // want `assigned value is dropped \(1 blank identifier\)`
	_ = x
}

func droppedIfInit() {
	if _, err := f(); err != nil { // want `assigned value is dropped \(1 blank identifier\)`
		return
	}
}

func droppedSwitchInit() {
	switch _, err := f(); { // want `assigned value is dropped \(1 blank identifier\)`
	case err != nil:
		return
	}
}

func droppedForInit() {
	for i, _ := f(); i < 10; i++ { // want `assigned value is dropped \(1 blank identifier\)`
		_ = i
	}
}

func rangeExempt() int {
	total := 0
	for _, v := range []int{1, 2} {
		total += v
	}
	return total
}

type iface interface{ M() }
type impl struct{}

func (impl) M() {}

var _ iface = impl{}

func exceptionWithReason() {
	_, _ = f() //nolint:droppedvalue -- f returns a value we do not need and an error checked elsewhere
}

func exceptionWithoutReason() {
	x, _ := f() //nolint:droppedvalue // want `droppedvalue marker requires a reason` `assigned value is dropped \(1 blank identifier\)`
	_ = x
}

func noDroppedValue() {
	x, err := f()
	if err != nil {
		return
	}
	_ = x
}

func strconvShape(s string) int64 {
	selected, _ := parseInt(s) // want `assigned value is dropped \(1 blank identifier\)`
	return selected
}

func dataShape() []byte {
	data, _ := readAll() // want `assigned value is dropped \(1 blank identifier\)`
	return data
}

func bleedOver() {
	_, _ = f()  //nolint:droppedvalue -- only the first drop is intended
	x, _ := f() // want `assigned value is dropped \(1 blank identifier\)`
	_ = x
}

func leadingMarker() {
	//nolint:droppedvalue -- standalone leading marker justifies the drop below
	_, _ = f()
}

func mapCommaOK() {
	m := map[string]int{"a": 1}
	_, ok := m["a"] // want `assigned value is dropped \(1 blank identifier\)`
	_ = ok
}

func commaListMarker() {
	_, _ = f() //nolint:gosec,droppedvalue -- unrelated and dropped
}

func commaListOther() {
	_, _ = f() //nolint:gosec -- unrelated reason // want `assigned value is dropped \(2 blank identifiers\)`
}

func multiLineAssign() {
	_, _ = multi(
		1,
		2,
	) //nolint:droppedvalue -- marker on the RHS line of a multi-line assignment
}

func blockCommentMarker() {
	_, _ = f() /* nolint:droppedvalue -- reason */ // want `assigned value is dropped \(2 blank identifiers\)`
}

func multiLineBleedOver() {
	_, _ = multi(
		1,
		2,
	) //nolint:droppedvalue -- only the first multi-line drop is intended
	x, _ := f() // want `assigned value is dropped \(1 blank identifier\)`
	_ = x
}
