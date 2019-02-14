// Package difff calculates the differences of document trees consisting of the
// standard go types created by unmarshaling from JSON, consisting of two
// complex types:
//   * map[string]interface{}
//   * []interface{}
// and five scalar types:
//   * string, int, float64, bool, nil
//
// difff is based off an algorithm designed for diffing XML documents outlined in:
//    Detecting Changes in XML Documents by Grégory Cobéna & Amélie Marian
//
// The paper describes an algorithm for generating an edit script that transitions
// between two states of tree-type data structures (XML). The general
// approach is as follows: For two given tree states, generate a diff script
// as a set of Deltas in 6 steps:
//
// 1. register in a map a unique signature (hash value) for every
//    subtree of the d1 (old) document
// 2. consider every subtree in d2 document, starting from the
//    largest. check if it is identitical to some the subtrees in
//    d1, if so match both subtrees.
// 3. attempt to match the parents of two matched subtrees
//    by checking labels (in our case, types of parent object or array)
//    controlling for bad matches based on length of path to the
//    ancestor and the weight of the matching subtrees. eg: a large
//    subtree may force the matching of its ancestors up to the root
//    a small subtree may not even force matching of its parent
// 4. Consider the largest subtrees of d2 in order. If one candidate
//    has it's parent already matched to the parent of the considered
//    node, it is certianly the best candidate.
// 5. At this point we might have matched all of d2. A node may not
//    match b/c its been inserted, or we missed matching it. We can now
//    do peephole optimization pass to retry some of the rejected nodes
//    once no more matchings can be obtained, unmatched nodes in d2
//    correspond to inserted nodes.
// 6. consider each matching node and decide if the node is at its right
//    place, or whether it has been moved.
package difff

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"hash"
	"hash/fnv"
	"sort"
	"strconv"
	"sync"
)

var (
	// ErrCompletelyDistinct is a holdover error while we work out the details
	ErrCompletelyDistinct = fmt.Errorf("these things are totally different")
)

// Diff computes a slice of deltas that define an edit script for turning the
// value at d1 into d2
func Diff(d1, d2 interface{}) ([]Delta, error) {
	var (
		wg        sync.WaitGroup
		t1, t2    Node
		t1Nodes   []Node
		t1nodesCh = make(chan Node)
		t2nodesCh = make(chan Node)
	)

	wg.Add(2)

	go func(nodes <-chan Node) {
		for n := range nodes {
			// t1SubTrees = sortAdd(n, t1SubTrees)
			t1Nodes = append(t1Nodes, n)
		}
	}(t1nodesCh)
	go func() {
		t1 = tree(d1, "", nil, t1nodesCh)
		close(t1nodesCh)
		wg.Done()
	}()

	go func(nodes <-chan Node) {
		for range nodes {
			// do nothing
		}
	}(t2nodesCh)
	go func() {
		t2 = tree(d2, "", nil, t2nodesCh)
		close(t2nodesCh)
		wg.Done()
	}()

	wg.Wait()

	// matches, err := findExactMatches(t1SubTrees, t2SubTrees)
	// if err != nil {
	// 	return nil, err
	// }

	matches := queueMatch(t1Nodes, t2)
	// fmt.Printf("%d %d\n", matches[0].left.Parent().Hash(), matches[0].right.Parent().Hash())
	for i, m := range matches {
		fmt.Printf("%d. %s %s\n", i, path(m[0]), path(m[1]))
	}

	fmt.Println(hex.EncodeToString(t1.Hash()), t1.Weight())
	fmt.Println(hex.EncodeToString(t2.Hash()), t2.Weight())
	// fmt.Println(t1SubTrees, t2SubTrees)
	for _, n := range t1Nodes {
		fmt.Printf("[%s %d] ", hex.EncodeToString(n.Hash()), n.Weight())
	}
	fmt.Printf("\n")

	// for _, n := range t2SubTrees {
	// 	fmt.Printf("[%s %d] ", hex.EncodeToString(n.Hash()), n.Weight())
	// }
	// fmt.Printf("\n")
	return nil, nil
}

// DeltaType defines the types of changes xydiff can create
// to describe the difference between two documents
type DeltaType uint8

const (
	// DTUnknown defaults DeltaType to undefined behaviour
	DTUnknown DeltaType = iota
	// DTRemove means making the children of a node
	// become the children of a node's parent
	DTRemove
	// DTInsert is the compliment of deleting, adding
	// children of a parent node to a new node, and making
	// that node a child of the original parent
	DTInsert
	// DTMove is the succession of a deletion & insertion
	// of the same node
	DTMove
	// DTChange is an alteration of a scalar data type (string, bool, float, etc)
	DTChange
)

// Delta represents a change between two documents
type Delta struct {
	Type DeltaType

	SrcPath []string
	DstPath []string

	SrcVal interface{}
	DstVal interface{}
}

func path(n Node) string {
	str := n.Name()
	for {
		n = n.Parent()
		if n == nil {
			break
		}
		str = fmt.Sprintf("%s.%s", n.Name(), str)
	}
	return str
}

// NewHash returns a new hash interface, wrapped in a function for easy
// hash algorithm switching, package consumers can override NewHash
// with their own desired hash.Hash implementation if the value space is
// particularly large. default is 32-bit FNV 1 for fast, cheap hashing
var NewHash = func() hash.Hash {
	return fnv.New32()
}

// NodeType defines all of the atoms in our universe
type NodeType uint8

const (
	// NTUnknown defines a type outside our universe, should never be encountered
	NTUnknown NodeType = iota
	// NTObject is a dictionary of key / value pairs
	NTObject
	NTArray
	NTString
	NTFloat
	NTInt
	NTBool
	NTNull
)

type Node interface {
	Type() NodeType
	Hash() []byte
	Weight() int
	Parent() Node
	Name() string
	Value() interface{}
}

type Compound interface {
	Children() []Node
}

type compound struct {
	t        NodeType
	name     string
	hash     []byte
	parent   Node
	children []Node
	weight   int
	value    interface{}
}

func (c compound) Type() NodeType     { return c.t }
func (c compound) Name() string       { return c.name }
func (c compound) Hash() []byte       { return c.hash }
func (c compound) Weight() int        { return c.weight }
func (c compound) Parent() Node       { return c.parent }
func (c compound) Value() interface{} { return c.value }
func (c compound) Children() []Node   { return c.children }

type scalar struct {
	t      NodeType
	name   string
	hash   []byte
	parent Node
	value  interface{}
}

func (s scalar) Type() NodeType     { return s.t }
func (s scalar) Name() string       { return s.name }
func (s scalar) Hash() []byte       { return s.hash }
func (s scalar) Weight() int        { return 1 }
func (s scalar) Parent() Node       { return s.parent }
func (s scalar) Value() interface{} { return s.value }

func tree(v interface{}, name string, parent Node, nodes chan Node) (n Node) {
	switch x := v.(type) {
	case nil:
		n = scalar{
			t:      NTNull,
			name:   name,
			hash:   NewHash().Sum([]byte("null")),
			parent: parent,
			value:  v,
		}
	case float64:
		fstr := strconv.FormatFloat(x, 'f', -1, 64)
		n = scalar{
			t:      NTFloat,
			name:   name,
			hash:   NewHash().Sum([]byte(fstr)),
			parent: parent,
			value:  v,
		}
	case string:
		n = scalar{
			t:      NTString,
			name:   name,
			hash:   NewHash().Sum([]byte(x)),
			parent: parent,
			value:  v,
		}
	case bool:
		bstr := "false"
		if x {
			bstr = "true"
		}
		n = scalar{
			t:     NTBool,
			name:  name,
			hash:  NewHash().Sum([]byte(bstr)),
			value: v,
		}
	case []interface{}:
		hasher := NewHash()
		arr := compound{
			t:      NTArray,
			name:   name,
			parent: parent,
			value:  v,
		}

		for i, v := range x {
			name := strconv.Itoa(i)
			node := tree(v, name, arr, nodes)
			hasher.Write(node.Hash())
			arr.children = append(arr.children, node)
		}
		arr.hash = hasher.Sum(nil)

		arr.weight = 1
		for _, ch := range arr.children {
			arr.weight += ch.Weight()
		}
		n = arr
	case map[string]interface{}:
		hasher := NewHash()
		obj := compound{
			t:      NTObject,
			name:   name,
			parent: parent,
			value:  v,
		}

		// gotta sort keys for consistent hashing :(
		names := make([]string, 0, len(x))
		for name := range x {
			names = append(names, name)
		}
		sort.Strings(names)

		for _, name := range names {
			node := tree(x[name], name, obj, nodes)
			hasher.Write(node.Hash())
			obj.children = append(obj.children, node)
		}
		obj.hash = hasher.Sum(nil)

		obj.weight = 1
		for _, ch := range obj.children {
			obj.weight += ch.Weight()
		}
		n = obj
	default:
		panic(fmt.Sprintf("unexpected type: %T", v))
	}

	nodes <- n
	return
}

// sortAdd inserts n into nodes, keeping the slice sorted by node weight,
// heaviest to the left
func sortAdd(n Node, nodes []Node) []Node {
	i := sort.Search(len(nodes), func(i int) bool { return nodes[i].Weight() <= n.Weight() })
	nodes = append(nodes, nil)
	copy(nodes[i+1:], nodes[i:])
	nodes[i] = n
	return nodes
}

// Match connects nodes from different trees
type Match [2]Node

func findExactMatches(t1SubTrees, t2SubTrees []Node) ([]Match, error) {
	// determine exact matches, starting top-down
	var matches []Match
	for _, n2 := range t2SubTrees {
		for _, n1 := range t1SubTrees {
			if bytes.Equal(n1.Hash(), n2.Hash()) {
				matches = append(matches, Match{n1, n2})
			}
		}
	}

	// TODO (b5) - wat do when no compounds are the same? leafs?
	if len(matches) == 0 {
		return nil, ErrCompletelyDistinct
	}

	return matches, nil
}

func queueMatch(t1Nodes []Node, t2 Node) (matches []Match) {
	queue := make(chan Node)
	done := make(chan struct{})
	considering := 1

	go func() {
		var candidates []Match
		for n2 := range queue {
			candidates = nil
			for _, n1 := range t1Nodes {
				if bytes.Equal(n1.Hash(), n2.Hash()) {
					candidates = append(candidates, Match{n1, n2})
				}
			}

			switch len(candidates) {
			case 0:
				// no candidates. check if node has children. adding if so.
				if n2c, ok := n2.(Compound); ok {
					for _, ch := range n2c.Children() {
						considering++
						go func(n Node) {
							queue <- n
						}(ch)
					}
				}
			case 1:
				matches = append(matches, candidates[0])
			default:
				match := bestCandidate(candidates)
				if match != nil {
					matches = append(matches, *match)
				}
			}

			considering--
			if considering == 0 {
				done <- struct{}{}
				break
			}
		}
	}()

	// start queue with t2 (root of tree)
	queue <- t2
	<-done
	return
}

// bestCandidate is the one who's parent
func bestCandidate(candidates []Match) *Match {
	// TODO
	return &candidates[0]
}
