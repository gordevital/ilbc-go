// Package cliflags lets commands accept flag-style options anywhere among
// positional arguments: it moves known valued options (and their values)
// ahead of the bare positionals before flag.Parse.
package cliflags

import "strings"

// Ordered returns args with all knownOptions (plus their values) moved to
// the front, preserving relative order inside each group.
func Ordered(args []string) []string {
	var head, tail []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		name := a
		inline := false
		if strings.HasPrefix(a, "-") {
			if eq := strings.IndexByte(a, '='); eq >= 0 {
				name = a[:eq]
				inline = true
			}
		}
		keyed, needValue := isKnown(name, inline)
		if keyed {
			head = append(head, a)
			if needValue {
				i++
				if i < len(args) {
					head = append(head, args[i])
				}
			}
		} else {
			tail = append(tail, a)
		}
	}
	return append(head, tail...)
}

func isKnown(name string, inline bool) (keyed, needValue bool) {
	if !strings.HasPrefix(name, "-") {
		return false, false
	}
	switch name {
	case "-ref", "-mode", "-pt", "-ssrc", "-sec", "-key", "-salt":
		return true, !inline
	}
	return false, false
}
