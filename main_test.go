package main

import (
	"testing"
	"time"
)

func TestLoadLocation(t *testing.T) {
	if loc, err := loadLocation("Local"); err != nil || loc != time.Local {
		t.Errorf(`loadLocation("Local") = %v, %v; want time.Local, nil`, loc, err)
	}
	if loc, err := loadLocation(""); err != nil || loc != time.Local {
		t.Errorf(`loadLocation("") = %v, %v; want time.Local, nil`, loc, err)
	}
	if loc, err := loadLocation("UTC"); err != nil || loc != time.UTC {
		t.Errorf(`loadLocation("UTC") = %v, %v; want time.UTC, nil`, loc, err)
	}
	// Proves the embedded time/tzdata resolves named zones (works on Windows too).
	if loc, err := loadLocation("Asia/Shanghai"); err != nil || loc.String() != "Asia/Shanghai" {
		t.Errorf(`loadLocation("Asia/Shanghai") = %v, %v; want Asia/Shanghai, nil`, loc, err)
	}
	if _, err := loadLocation("Not/ARealZone"); err == nil {
		t.Error(`loadLocation("Not/ARealZone") = nil error; want an error`)
	}
}
