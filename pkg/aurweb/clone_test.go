package aurweb

import (
	"reflect"
	"testing"
)

func TestPkgCloneOwnsEveryRelation(t *testing.T) {
	var original Pkg
	value := reflect.ValueOf(&original).Elem()
	for i := 0; i < value.NumField(); i++ {
		if field := value.Field(i); field.Kind() == reflect.Slice {
			field.Set(reflect.ValueOf([]string{"original"}))
		}
	}
	clone := original.Clone()
	copyValue := reflect.ValueOf(&clone).Elem()
	for i := 0; i < value.NumField(); i++ {
		if field := copyValue.Field(i); field.Kind() == reflect.Slice {
			field.Index(0).SetString("changed")
			if got := value.Field(i).Index(0).String(); got != "original" {
				t.Errorf("Clone shares %s: %q", value.Type().Field(i).Name, got)
			}
		}
	}
	if zero := (Pkg{}).Clone(); zero.Depends != nil || zero.License != nil {
		t.Fatal("Clone changed nil arrays")
	}
}
