package aiassistant

import "testing"

func TestOrDefault(t *testing.T) {
	if got := orDefault("hello", "default"); got != "hello" {
		t.Errorf("orDefault('hello','default') = %v", got)
	}
	if got := orDefault("", "default"); got != "default" {
		t.Errorf("orDefault('','default') = %v，期望 'default'", got)
	}
}

func TestTruncate(t *testing.T) {
	if got := truncate("hello world", 5); got != "hello" {
		t.Errorf("truncate('hello world',5) = %q，期望 'hello'", got)
	}
	if got := truncate("hi", 10); got != "hi" {
		t.Errorf("truncate('hi',10) = %q，期望 'hi'", got)
	}
	if got := truncate("", 5); got != "" {
		t.Errorf("truncate('',5) = %q", got)
	}
}
