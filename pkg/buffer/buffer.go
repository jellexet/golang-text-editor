package buffer

type Buffer interface {
	Insert(idx int, s string)
	Delete(idx int, len int)
	String() string
}
