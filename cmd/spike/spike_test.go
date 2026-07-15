package main

import "testing"

func TestParseControl(t *testing.T) {
	msg, err := parseControl([]byte(`{"t":"in","d":"ls\r"}`))
	if err != nil || msg.T != "in" || msg.D != "ls\r" {
		t.Fatalf("input frame: %+v, %v", msg, err)
	}
	msg, err = parseControl([]byte(`{"t":"rs","c":120,"r":40}`))
	if err != nil || msg.Cols != 120 || msg.Rows != 40 {
		t.Fatalf("resize frame: %+v, %v", msg, err)
	}
	if _, err := parseControl([]byte(`{"t":"nope"}`)); err == nil {
		t.Fatal("want error on unknown type")
	}
	if _, err := parseControl([]byte(`garbage`)); err == nil {
		t.Fatal("want error on invalid json")
	}
}
