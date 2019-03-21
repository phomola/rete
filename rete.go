// Copyright 2019 Petr Homola. All rights reserved.
// Use of this source code is governed by the AGPL v3.0
// that can be found in the LICENSE file.

package rete

import (
	"fmt"
	"strings"
)

type Tuple struct {
	comps []string
}

func NewTuple(comps ...string) *Tuple {
	return &Tuple{comps}
}

func (t *Tuple) Get(i int) string {
	return t.comps[i]
}

func (t *Tuple) Equals(t2 *Tuple) bool {
	if len(t.comps) == len(t2.comps) {
		for i, c1 := range t.comps {
			c2 := t2.comps[i]
			if c1 != c2 {
				return false
			}
		}
		return true
	}
	return false
}

func (t *Tuple) String() string {
	return "(" + strings.Join(t.comps, ",") + ")"
}

type Sequence struct {
	tuples []*Tuple
}

func NewSequence(tuples ...*Tuple) *Sequence {
	return &Sequence{tuples}
}

func (seq *Sequence) Appending(seq2 *Sequence) *Sequence {
	t1 := seq.tuples[0:len(seq.tuples):len(seq.tuples)]
	return &Sequence{append(t1, seq2.tuples...)}
}

func (seq *Sequence) Get(i int) *Tuple {
	return seq.tuples[i]
}

func (seq1 *Sequence) Equals(seq2 *Sequence) bool {
	if len(seq1.tuples) == len(seq2.tuples) {
		for i, t1 := range seq1.tuples {
			t2 := seq2.tuples[i]
			if !t1.Equals(t2) {
				return false
			}
		}
		return true
	}
	return false
}

func (seq *Sequence) String() string {
	b := strings.Builder{}
	b.WriteString("[")
	for i, t := range seq.tuples {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(t.String())
	}
	b.WriteString("]")
	return b.String()
}

type Node interface {
	EnumSequences(cb func(*Sequence))
}

type alphaIndexKey struct {
	pos   int
	value string
}

type targetNode struct {
	node  *BetaNode
	index int
}

type AlphaNode struct {
	sig     string
	tuples  []*Tuple
	indices map[alphaIndexKey][]*Tuple
	targets []targetNode
	actions []func(*Sequence)
}

func NewAlphaNode(sig string) *AlphaNode {
	return &AlphaNode{sig, nil, make(map[alphaIndexKey][]*Tuple), nil, nil}
}

func (node *AlphaNode) AddAction(a func(*Sequence)) {
	node.actions = append(node.actions, a)
}

func (node *AlphaNode) EnumSequences(cb func(*Sequence)) {
	for _, t := range node.tuples {
		cb(NewSequence(t))
	}
}

func (node *AlphaNode) AddTuple(tuple *Tuple) bool {
	key := alphaIndexKey{0, tuple.comps[0]}
	if index, ok := node.indices[key]; ok {
		for _, tuple2 := range index {
			if tuple.Equals(tuple2) {
				return false
			}
		}
	}
	node.tuples = append(node.tuples, tuple)
	for i, comp := range tuple.comps {
		key := alphaIndexKey{i, comp}
		index := node.indices[key]
		index = append(index, tuple)
		node.indices[key] = index
	}
	seq := NewSequence(tuple)
	for _, t := range node.targets {
		t.node.Notify(t.index, seq)
	}
	for _, a := range node.actions {
		a(seq)
	}
	return true
}

func (node *AlphaNode) AddTarget(t *BetaNode, index int) {
	node.targets = append(node.targets, targetNode{t, index})
}

type betaIndexKey struct {
	pos1  int
	pos2  int
	value string
}

type Binding struct {
	Tuple1 int
	Comp1  int
	Tuple2 int
	Comp2  int
}

type BetaNode struct {
	sources   []Node
	actions   []func(*Sequence)
	sequences []*Sequence
	indices   map[betaIndexKey][]*Sequence
	bindings  []Binding
	targets   []targetNode
}

func NewBetaNode() *BetaNode {
	return &BetaNode{indices: make(map[betaIndexKey][]*Sequence)}
}

func (node *BetaNode) AddBinding(b Binding) {
	node.bindings = append(node.bindings, b)
}

func (node *BetaNode) AddTarget(t *BetaNode, index int) {
	node.targets = append(node.targets, targetNode{t, index})
}

func (node *BetaNode) AddSequence(seq *Sequence) bool {
	key := betaIndexKey{0, 0, seq.tuples[0].comps[0]}
	if index, ok := node.indices[key]; ok {
		for _, seq2 := range index {
			if seq.Equals(seq2) {
				return false
			}
		}
	}
	node.sequences = append(node.sequences, seq)
	for i, tuple := range seq.tuples {
		for j, comp := range tuple.comps {
			key := betaIndexKey{i, j, comp}
			index := node.indices[key]
			index = append(index, seq)
			node.indices[key] = index
		}
	}
	for _, t := range node.targets {
		t.node.Notify(t.index, seq)
	}
	return true
}

func (node *BetaNode) AddSource(n Node) {
	node.sources = append(node.sources, n)
}

func (node *BetaNode) AddAction(a func(*Sequence)) {
	node.actions = append(node.actions, a)
}

func checkBinding(seq1, seq2 *Sequence, b Binding) bool {
	return seq1.Get(b.Tuple1).Get(b.Comp1) == seq2.Get(b.Tuple2).Get(b.Comp2)
}

func (node *BetaNode) Notify(index int, seq *Sequence) {
	node2 := node.sources[1-index]
	node2.EnumSequences(func(seq2 *Sequence) {
		cons := true
		for _, b := range node.bindings {
			var cons2 bool
			if index == 0 {
				cons2 = checkBinding(seq, seq2, b)
			} else {
				cons2 = checkBinding(seq2, seq, b)
			}
			if !cons2 {
				cons = false
				break
			}
		}
		if cons {
			var newSeq *Sequence
			if index == 0 {
				newSeq = seq.Appending(seq2)
			} else {
				newSeq = seq2.Appending(seq)
			}
			if node.AddSequence(newSeq) {
				node.performActions(newSeq)
			}
		}
	})
}

func (node *BetaNode) performActions(seq *Sequence) {
	for _, a := range node.actions {
		a(seq)
	}
}

func (node *BetaNode) EnumSequences(cb func(*Sequence)) {
	for _, s := range node.sequences {
		cb(s)
	}
}

type Network struct {
	alphaNodes map[string]*AlphaNode
}

func NewNetwork() *Network {
	return &Network{make(map[string]*AlphaNode)}
}

func (net *Network) AlphaNode(sig string) *AlphaNode {
	node, ok := net.alphaNodes[sig]
	if !ok {
		node = NewAlphaNode(sig)
		net.alphaNodes[sig] = node
	}
	return node
}

func (net *Network) AddNode(node Node) {
	switch node := node.(type) {
	case *AlphaNode:
		net.alphaNodes[node.sig] = node
	}
}

func (net *Network) AddTuple(functor string, comps ...string) {
	sig := fmt.Sprintf("%s/%d", functor, len(comps))
	node, ok := net.alphaNodes[sig]
	if !ok {
		node = NewAlphaNode(sig)
		net.alphaNodes[sig] = node
	}
	tuple := NewTuple(comps...)
	node.AddTuple(tuple)
}

func (net *Network) String() string {
	sb := strings.Builder{}
	for sig, node := range net.alphaNodes {
		fmt.Println(sig)
		for _, tuple := range node.tuples {
			fmt.Printf(" %s\n", tuple)
		}
	}
	return sb.String()
}
