package commands

import "testing"

func TestRegistryDispatch(t *testing.T) {
	registry, err := New([]Command{{Name: "/test", Description: "Test.", Action: Exit}})
	if err != nil {
		t.Fatal(err)
	}
	command, ok := registry.Lookup("/test")
	if !ok || command.Action != Exit {
		t.Fatalf("lookup = %#v, %v", command, ok)
	}
	if _, ok := registry.Lookup("hello"); ok {
		t.Fatal("normal chat input matched a command")
	}
}

func TestRegistryRejectsDuplicates(t *testing.T) {
	_, err := New([]Command{{Name: "/same"}, {Name: "/same"}})
	if err == nil {
		t.Fatal("expected duplicate command error")
	}
}
