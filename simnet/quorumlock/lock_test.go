package quorumlock

import (
	"testing"
	"time"
)

func TestLock(t *testing.T) {
	l := New(time.Second)

	if l.HeldBy(0) || l.HeldBy(1) || l.HeldBy(7) {
		t.Error("Lock incorrectly reports that a transaction holds it.")
	}

	l.Unlock(5)
	// Should not error

	if l.HeldBy(0) || l.HeldBy(1) || l.HeldBy(7) {
		t.Error("Lock incorrectly reports that a transaction holds it.")
	}

	l.Lock(3)

	if l.HeldBy(0) || l.HeldBy(1) || l.HeldBy(7) {
		t.Error("Lock incorrectly reports that a wrong transaction holds it.")
	}

	if !l.HeldBy(3) {
		t.Error("Lock is not correctly locked.")
	}

	if l.Lock(4) {
		t.Error("Lock can be stolen.")
	}

	l.Unlock(4)

	if !l.HeldBy(3) {
		t.Error("Lock is wrongly lost.")
	}

	l.Unlock(3)

	if l.HeldBy(3) {
		t.Error("Lock not correctly unlocked.")
	}

	if !l.Lock(2) {
		t.Error("Lock cannot be relocked.")
	}
}
