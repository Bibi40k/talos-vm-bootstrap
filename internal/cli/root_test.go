package cli

import "testing"

func TestNewLoggerValidation(t *testing.T) {
	if _, err := newLogger("text", "info"); err != nil {
		t.Fatalf("expected valid logger: %v", err)
	}
	if _, err := newLogger("invalid", "info"); err == nil {
		t.Fatalf("expected invalid format error")
	}
	if _, err := newLogger("text", "invalid"); err == nil {
		t.Fatalf("expected invalid level error")
	}
}

func TestUserError(t *testing.T) {
	e := &userError{msg: "boom", hint: "try again"}
	if e.Error() != "boom" {
		t.Fatalf("unexpected msg: %q", e.Error())
	}
	if e.Hint() != "try again" {
		t.Fatalf("unexpected hint: %q", e.Hint())
	}
}
