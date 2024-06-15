package helper

import (
	"reflect"
	"testing"
)

func TestPtr(t *testing.T) {
	type tt string

	const TT tt = "hi"
	TT2 := Ptr(TT)
	if *TT2 != TT {
		t.Errorf("Ptr string const failed")
	}

	s := "a"
	sp := Ptr(s)
	if *sp != s {
		t.Errorf("Ptr[string] failed")
	}

	i := 2
	ip := Ptr(i)
	if *ip != i {
		t.Errorf("Ptr[int] failed")
	}
}

func TestDeref(t *testing.T) {
	// Define a struct for user-defined types.
	type MyStruct struct {
		Value int
	}

	// Test dereferencing an int pointer.
	intValue := 42
	intPtr := &intValue
	derefInt := Deref(intPtr)
	if derefInt != intValue {
		t.Errorf("Expected %d, but got %d", intValue, derefInt)
	}

	// Test dereferencing a float64 pointer.
	floatValue := 3.14
	floatPtr := &floatValue
	derefFloat := Deref(floatPtr)
	if derefFloat != floatValue {
		t.Errorf("Expected %f, but got %f", floatValue, derefFloat)
	}

	// Test dereferencing a string pointer.
	strValue := "Hello, World!"
	strPtr := &strValue
	derefStr := Deref(strPtr)
	if derefStr != strValue {
		t.Errorf("Expected %s, but got %s", strValue, derefStr)
	}

	// Test dereferencing a nil pointer.
	var nilIntPtr *int
	derefNilInt := Deref(nilIntPtr)
	if derefNilInt != 0 {
		t.Errorf("Expected 0, but got %d", derefNilInt)
	}

	// Test dereferencing a pointer to a user-defined struct.
	myStructValue := MyStruct{Value: 100}
	myStructPtr := &myStructValue
	derefMyStruct := Deref(myStructPtr)
	if !reflect.DeepEqual(derefMyStruct, myStructValue) {
		t.Errorf("Expected %+v, but got %+v", myStructValue, derefMyStruct)
	}

	// Test dereferencing a pointer to a nil user-defined struct.
	var myStruct2Value MyStruct
	myStruct2Ptr := &myStruct2Value
	derefMyStruct2 := Deref(myStruct2Ptr)
	if !reflect.DeepEqual(derefMyStruct2, myStruct2Value) {
		t.Errorf("Expected %+v, but got %+v", myStruct2Value, derefMyStruct2)
	}
}
