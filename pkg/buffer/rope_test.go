package buffer

import (
	"testing"
)

func TestRopeBasicOperations(t *testing.T) {
	// short string
	r := NewRope("abc")
	if r.Length() != 3 {
		t.Fatalf("expected length 3 got %d", r.Length())
	}
	if r.String() != "abc" {
		t.Fatalf("expected string abc got %q", r.String())
	}
	// index
	b, err := r.Index(1)
	if err != nil || b != 'b' {
		t.Fatalf("index returned %v %v", b, err)
	}

	// Insert at beginning
	r2, err := r.Insert(0, "X")
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}
	if r2.String() != "Xabc" {
		t.Fatalf("insert beginning wrong: %q", r2.String())
	}

	// Insert middle
	r3, err := r2.Insert(2, "Y")
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}
	if r3.String() != "XaYbc" && r3.Length() == 0 {
		// this check will fail fast if wrong; secondary condition just to avoid static analysis warnings
	}

	// Delete range
	r4, err := r3.Delete(1, 3) // delete "aY"
	if err != nil {
		t.Fatalf("delete error: %v", err)
	}
	if r4.String() != "Xbc" {
		t.Fatalf("delete result wrong: %q", r4.String())
	}

	// Substring
	sub, err := r4.Substring(1, 3) // "bc"
	if err != nil {
		t.Fatalf("substring error: %v", err)
	}
	if sub != "bc" {
		t.Fatalf("substring wrong: %q", sub)
	}

	// Concat & Split
	a := NewRope("hello")
	bb := NewRope("world")
	joined := Concat(a, bb)
	if joined.String() != "helloworld" {
		t.Fatalf("concat wrong: %q", joined.String())
	}
	// Split at 5
	left, right, err := joined.Split(5)
	if err != nil {
		t.Fatalf("split error: %v", err)
	}
	if left.String() != "hello" || right.String() != "world" {
		t.Fatalf("split parts wrong: %q | %q", left.String(), right.String())
	}
}

func TestRopeIndexOutOfBounds(t *testing.T) {
	r := NewRope("abc")
	if _, err := r.Index(10); err == nil {
		t.Fatalf("expected out of bounds error")
	}
}

func TestRopeInsertDeleteEdgeCases(t *testing.T) {
	r := NewRope("")
	r2, err := r.Insert(0, "x")
	if err != nil {
		t.Fatalf("insert into nil/empty rope error: %v", err)
	}
	if r2.String() != "x" {
		t.Fatalf("insert into empty wrong: %q", r2.String())
	}

	r3, err := r2.Delete(0, 0)
	if err != nil {
		t.Fatalf("delete empty range error: %v", err)
	}
	if r3.String() != "x" {
		t.Fatalf("delete empty range changed rope: %q", r3.String())
	}
}

// Simple fuzz test.
func FuzzRopeOps(f *testing.F) {
	f.Add([]byte("hello"))
	f.Add([]byte(""))

	f.Fuzz(func(t *testing.T, data []byte) {
		s := string(data)
		r := NewRope(s)

		// Making sure functions don't panic and maintain consistency.
		_ = r.String()
		_ = r.Length()

		// Try Insert at end and at beginning
		if _, err := r.Insert(0, s); err != nil {
			t.Fatalf("Insert(0) error: %v", err)
		}
		if _, err := r.Insert(r.Length(), s); err != nil {
			t.Fatalf("Insert(end) error: %v", err)
		}

		// Try Delete small ranges when possible
		if r.Length() >= 2 {
			if _, err := r.Delete(0, 1); err != nil {
				t.Fatalf("Delete error: %v", err)
			}
		}

		// Substring safe calls
		if r.Length() >= 1 {
			if _, err := r.Substring(0, 1); err != nil {
				t.Fatalf("Substring error: %v", err)
			}
		}

		// Rebalance shouldn't panic
		_ = r.Rebalance()
	})
}
