package main

import (
    "encoding/json"
    "fmt"
)

type StringSet struct {
    table map[string]bool
}

func NewStringSet() *StringSet {
    return &StringSet{make(map[string]bool)}
}

func (ss *StringSet) Add(s string) {
    ss.table[s] = true
}

func (ss *StringSet) Remove(s string) {
    delete(ss.table, s)
}

func (ss *StringSet) Contain(s string) bool {
    return ss.table[s]
}

func (ss *StringSet) Size() int {
    return len(ss.table)
}

func (ss *StringSet) Equal(s2 *StringSet) bool {
    if ss.Size() != s2.Size() {
        return false
    }

    for k := range ss.table {
        if !s2.Contain(k) {
            return false
        }
    }
    return true
}

func (ss *StringSet) Values() []string {
    i, vs := 0, make([]string, len(ss.table))
    for k := range ss.table {
        vs[i] = k
        i++
    }
    return vs
}

func (ss *StringSet) MarshalJSON() ([]byte, error) {
    return json.Marshal(ss.Values())
}

func (ss *StringSet) String() string {
    return fmt.Sprintf("%v", ss.Values())
}
