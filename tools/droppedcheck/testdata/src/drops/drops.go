package drops

import "errors"

func f() (int, error)      { return 1, errors.New("x") }
func g() (int, error)      { return 1, errors.New("y") }
func parseInt(s string) (int64, error) { return 1, nil }
func readAll() ([]byte, error)         { return nil, nil }

func droppedErrorShortVar() {
	x, _ := f() // want `assigned value is dropped \(1 blank identifier\(s\)\)`
	_ = x
}

func droppedNonErrorValue() {
	_, err := f() // want `assigned value is dropped \(1 blank identifier\(s\)\)`
	if err != nil {
		return
	}
}

func droppedBoth() {
	_, _ = f() // want `assigned value is dropped \(2 blank identifier\(s\)\)`
}

func droppedSingleAssign() {
	var x int
	x, _ = f() // want `assigned value is dropped \(1 blank identifier\(s\)\)`
	_ = x
}

func droppedIfInit() {
	if _, err := f(); err != nil { // want `assigned value is dropped \(1 blank identifier\(s\)\)`
		return
	}
}

func droppedSwitchInit() {
	switch _, err := f(); { // want `assigned value is dropped \(1 blank identifier\(s\)\)`
	case err != nil:
		return
	}
}

func droppedForInit() {
	for i, _ := f(); i < 10; i++ { // want `assigned value is dropped \(1 blank identifier\(s\)\)`
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
	x, _ := f() //nolint:droppedvalue // want `droppedvalue requires a reason` `assigned value is dropped \(1 blank identifier\(s\)\)`
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
	selected, _ := parseInt(s) // want `assigned value is dropped \(1 blank identifier\(s\)\)`
	return selected
}

func dataShape() []byte {
	data, _ := readAll() // want `assigned value is dropped \(1 blank identifier\(s\)\)`
	return data
}