package main

import (
	"reflect"
	"testing"
)

func TestExtractDBFlag(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		wantDB   string
		wantRest []string
	}{
		{name: "space form", args: []string{"--db", "/tmp/x.db", "registry", "list"}, wantDB: "/tmp/x.db", wantRest: []string{"registry", "list"}},
		{name: "equals form", args: []string{"--db=/tmp/x.db", "registry", "list"}, wantDB: "/tmp/x.db", wantRest: []string{"registry", "list"}},
		{name: "no flag", args: []string{"registry", "list"}, wantDB: "", wantRest: []string{"registry", "list"}},
		{name: "flag without value", args: []string{"registry", "--db"}, wantDB: "", wantRest: []string{"registry"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, rest := extractDBFlag(tt.args)
			if db != tt.wantDB {
				t.Errorf("extractDBFlag(%v) db = %q, want %q", tt.args, db, tt.wantDB)
			}
			if !reflect.DeepEqual(rest, tt.wantRest) {
				t.Errorf("extractDBFlag(%v) rest = %v, want %v", tt.args, rest, tt.wantRest)
			}
		})
	}
}
