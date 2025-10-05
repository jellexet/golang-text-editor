package main

import (
	"fmt"
	"strings"
)

// Rope is the main data structure for the text editor
type Rope struct {
	left   *Rope
	right  *Rope
	data   string
	weight int // length of string in left subtree
	length int // total length of this rope
}

const (
	maxLeafLength = 8 // maximum length of a leaf node
)

// NewRope creates a new rope from a string
func NewRope(s string) *Rope {
	if len(s) <= maxLeafLength {
		return &Rope{
			data:   s,
			weight: len(s),
			length: len(s),
		}
	}

	// Split the string in the middle
	mid := len(s) / 2
	left := NewRope(s[:mid])
	right := NewRope(s[mid:])

	return &Rope{
		left:   left,
		right:  right,
		weight: left.length,
		length: left.length + right.length,
	}
}

// String converts the rope back to a string
func (r *Rope) String() string {
	if r == nil {
		return ""
	}
	if r.isLeaf() {
		return r.data
	}
	return r.left.String() + r.right.String()
}

// isLeaf checks if the node is a leaf
func (r *Rope) isLeaf() bool {
	return r.left == nil && r.right == nil
}

// Index returns the character at the given index
func (r *Rope) Index(i int) (byte, error) {
	if i < 0 || i >= r.length {
		return 0, fmt.Errorf("index %d out of bounds [0, %d)", i, r.length)
	}

	if r.isLeaf() {
		return r.data[i], nil
	}

	if i < r.weight {
		return r.left.Index(i)
	}
	return r.right.Index(i - r.weight)
}

// Concat concatenates two ropes
func Concat(r1, r2 *Rope) *Rope {
	if r1 == nil {
		return r2
	}
	if r2 == nil {
		return r1
	}

	return &Rope{
		left:   r1,
		right:  r2,
		weight: r1.length,
		length: r1.length + r2.length,
	}
}

// Split splits the rope at the given index
func (r *Rope) Split(i int) (*Rope, *Rope, error) {
	if i < 0 || i > r.length {
		return nil, nil, fmt.Errorf("index %d out of bounds [0, %d]", i, r.length)
	}

	if i == 0 {
		return nil, r, nil
	}
	if i == r.length {
		return r, nil, nil
	}

	if r.isLeaf() {
		left := NewRope(r.data[:i])
		right := NewRope(r.data[i:])
		return left, right, nil
	}

	if i < r.weight {
		left, right, err := r.left.Split(i)
		if err != nil {
			return nil, nil, err
		}
		return left, Concat(right, r.right), nil
	} else if i > r.weight {
		left, right, err := r.right.Split(i - r.weight)
		if err != nil {
			return nil, nil, err
		}
		return Concat(r.left, left), right, nil
	}

	return r.left, r.right, nil
}

// Insert inserts a string at the given index
func (r *Rope) Insert(i int, s string) (*Rope, error) {
	if i < 0 || i > r.length {
		return nil, fmt.Errorf("index %d out of bounds [0, %d]", i, r.length)
	}

	left, right, err := r.Split(i)
	if err != nil {
		return nil, err
	}

	newRope := NewRope(s)
	return Concat(Concat(left, newRope), right), nil
}

// Delete deletes characters from start to end (exclusive)
func (r *Rope) Delete(start, end int) (*Rope, error) {
	if start < 0 || end > r.length || start > end {
		return nil, fmt.Errorf("invalid range [%d, %d) for length %d", start, end, r.length)
	}

	if start == end {
		return r, nil
	}

	left, temp, err := r.Split(start)
	if err != nil {
		return nil, err
	}

	_, right, err := temp.Split(end - start)
	if err != nil {
		return nil, err
	}

	return Concat(left, right), nil
}

// Substring returns a substring from start to end (exclusive)
func (r *Rope) Substring(start, end int) (string, error) {
	if start < 0 || end > r.length || start > end {
		return "", fmt.Errorf("invalid range [%d, %d) for length %d", start, end, r.length)
	}

	if start == end {
		return "", nil
	}

	_, temp, err := r.Split(start)
	if err != nil {
		return "", err
	}

	sub, _, err := temp.Split(end - start)
	if err != nil {
		return "", err
	}

	return sub.String(), nil
}

// Length returns the total length of the rope
func (r *Rope) Length() int {
	if r == nil {
		return 0
	}
	return r.length
}

// Print displays the rope structure (for debugging)
func (r *Rope) Print(indent string) {
	if r == nil {
		return
	}
	if r.isLeaf() {
		fmt.Printf("%sLeaf: \"%s\" (len=%d)\n", indent, r.data, r.length)
		return
	}
	fmt.Printf("%sNode: weight=%d, length=%d\n", indent, r.weight, r.length)
	r.left.Print(indent + "  L:")
	r.right.Print(indent + "  R:")
}

// Example usage
//rope := NewRope("Hello, World!")
//fmt.Println("Original:", rope.String())
//fmt.Println("Length:", rope.Length())

// Index access
//if ch, err := rope.Index(7); err == nil {
//	fmt.Printf("Character at index 7: %c\n", ch)
//}

// Insert
//rope, _ = rope.Insert(7, "Beautiful ")
//fmt.Println("After insert:", rope.String())

// Substring
//if sub, err := rope.Substring(7, 16); err == nil {
//	fmt.Println("Substring [7:16]:", sub)
//}

// Delete
//rope, _ = rope.Delete(7, 16)
//fmt.Println("After delete:", rope.String())

// Concatenation
//rope2 := NewRope(" How are you?")
//rope = Concat(rope, rope2)
//fmt.Println("After concat:", rope.String())

// Print structure
//fmt.Println("\nRope structure:")
//rope.Print("")
