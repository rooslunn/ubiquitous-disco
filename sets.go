package main

type Set struct {
	m map[any]struct{}
}

func NewSet(items ...any) *Set {
	s := &Set{
		m: make(map[interface{}]struct{}),
	}
	s.Add(items...)
	return s
}

func (s *Set) Add(items ...any) {
	for _, item := range items {
		s.m[item] = struct{}{}
	}
}

func (s *Set) Remove(item any) {
	delete(s.m, item)
}

func (s *Set) Contains(item any) bool {
	_, ok := s.m[item]
	return ok
}

func (s *Set) Size() int {
	return len(s.m)
}

// func main() {
// mySet := NewSet("apple", "banana", "orange")
// mySet.Add("grape", "banana") // "banana" is already present, so no duplicate is added

// fmt.Println("Set contains 'apple':", mySet.Contains("apple"))    // true
// fmt.Println("Set contains 'kiwi':", mySet.Contains("kiwi"))      // false
// fmt.Println("Set size:", mySet.Size())                           // 4

// mySet.Remove("banana")
// fmt.Println("Set contains 'banana':", mySet.Contains("banana")) // false
// fmt.Println("Set size after removal:", mySet.Size())            // 3
// }
