package buffer

import (
	"fmt"
)

// Rope data structure - a binary tree for efficient text manipulation
type Rope struct {
	left   *Rope
	right  *Rope
	data   string
	weight int
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
		}
	}

	// Split the string in the middle
	mid := len(s) / 2
	left := NewRope(s[:mid])
	right := NewRope(s[mid:])

	return &Rope{
		left:   left,
		right:  right,
		weight: left.Length(),
	}
}

// Weight returns the weight of this node (length of all leaves in left subtree)
func (r *Rope) Weight() int {
	if r == nil {
		return 0
	}
	return r.weight
}

// Length returns the total length of the rope
func (r *Rope) Length() int {
	if r == nil {
		return 0
	}
	if r.isLeaf() {
		return len(r.data)
	}
	return r.left.Length() + r.right.Length()
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
	if r == nil {
		return 0, fmt.Errorf("rope is nil")
	}

	length := r.Length()
	if i < 0 || i >= length {
		return 0, fmt.Errorf("index %d out of bounds [0, %d)", i, length)
	}

	if r.isLeaf() {
		return r.data[i], nil
	}

	// If index is less than weight, go to left subtree
	if i < r.weight {
		return r.left.Index(i)
	}
	// Otherwise, go to right subtree with adjusted index
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
		weight: r1.Length(), // weight is total length of left subtree
	}
}

// Split splits the rope at the given index into two ropes
func (r *Rope) Split(i int) (*Rope, *Rope, error) {
	if r == nil {
		return nil, nil, fmt.Errorf("rope is nil")
	}

	length := r.Length()
	if i < 0 || i > length {
		return nil, nil, fmt.Errorf("index %d out of bounds [0, %d]", i, length)
	}

	if i == 0 {
		return nil, r, nil
	}
	if i == length {
		return r, nil, nil
	}

	if r.isLeaf() {
		// Split point is in the middle of a leaf string
		left := NewRope(r.data[:i])
		right := NewRope(r.data[i:])
		return left, right, nil
	}

	if i < r.weight {
		// Split point is in left subtree
		left, right, err := r.left.Split(i)
		if err != nil {
			return nil, nil, err
		}
		// Concatenate the right part of left subtree with the entire right subtree
		return left, Concat(right, r.right), nil
	} else if i > r.weight {
		// Split point is in right subtree
		left, right, err := r.right.Split(i - r.weight)
		if err != nil {
			return nil, nil, err
		}
		// Concatenate the entire left subtree with left part of right subtree
		return Concat(r.left, left), right, nil
	}

	// Split point is exactly at the boundary
	return r.left, r.right, nil
}

// Insert inserts a string at the given index - returns new rope
func (r *Rope) Insert(i int, s string) (*Rope, error) {
	if r == nil {
		return NewRope(s), nil
	}

	length := r.Length()
	if i < 0 || i > length {
		return nil, fmt.Errorf("index %d out of bounds [0, %d]", i, length)
	}

	left, right, err := r.Split(i)
	if err != nil {
		return nil, err
	}

	newRope := NewRope(s)
	return Concat(Concat(left, newRope), right), nil
}

// Delete deletes characters from start to end (exclusive) - returns new rope
func (r *Rope) Delete(start, end int) (*Rope, error) {
	if r == nil {
		return nil, fmt.Errorf("rope is nil")
	}

	length := r.Length()
	if start < 0 || end > length || start > end {
		return nil, fmt.Errorf("invalid range [%d, %d) for length %d", start, end, length)
	}

	if start == end {
		return r, nil
	}

	// Split into three parts: before start, [start:end], after end
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
	if r == nil {
		return "", fmt.Errorf("rope is nil")
	}

	length := r.Length()
	if start < 0 || end > length || start > end {
		return "", fmt.Errorf("invalid range [%d, %d) for length %d", start, end, length)
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

// Print displays the rope structure (for debugging)
func (r *Rope) Print(indent string) {
	if r == nil {
		fmt.Printf("%s<nil>\n", indent)
		return
	}
	if r.isLeaf() {
		fmt.Printf("%sLeaf: \"%s\" (len=%d, weight=%d)\n", indent, r.data, len(r.data), r.weight)
		return
	}
	fmt.Printf("%sNode: weight=%d, total_length=%d\n", indent, r.weight, r.Length())
	r.left.Print(indent + "  L:")
	r.right.Print(indent + "  R:")
}

// Rebalance optimizes the rope structure (optional, for maintaining performance)
func (r *Rope) Rebalance() *Rope {
	if r == nil {
		return nil
	}
	// Simple rebalancing: convert to string and rebuild
	return NewRope(r.String())
}
