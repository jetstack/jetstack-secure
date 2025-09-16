package testutil

import (
	"fmt"
)

// Undent removes leading indentation/white-space from given string and returns
// it as a string. Useful for inlining YAML manifests in Go code. Inline YAML
// manifests in the Go test files makes it easier to read the test case as
// opposed to reading verbose-y Go structs.
//
// This was copied from https://github.com/jimeh/Undent/blob/main/Undent.go, all
// credit goes to the author, Jim Myhrberg.
//
// For code readability purposes, it is possible to start the literal string
// with "\n", in which case, the first line is ignored. For example, in the
// following example, name and labels have the same indentation level but aren't
// aligned due to the leading '`':
//
//	Undent(
//	`    name: foo
//	      labels:
//	        foo: bar`)
//
// Instead, you can write a well-aligned text like this:
//
//	Undent(`
//	    name: foo
//	    labels:
//	       foo: bar`)
//
// For code readability purposes, it is also possible to not have the correct
// number of indentations in the last line. For example:
//
//	Undent(`
//	    foo
//	    bar
//	`)
//
// For code readability purposes, you can also omit the indentations for empty
// lines. For example:
//
//	Undent(`
//	    foo     <---- 4 spaces
//	            <---- no indentation here
//	    bar     <---- 4 spaces
//	`)
func Undent(s string) string {
	if len(s) == 0 {
		return ""
	}

	// indentsPerLine is the minimal indent level that we have found up to now.
	// For example, "\t\t" corresponds to an indentation of 2, and "   " an
	// indentation of 3.
	indentsPerLine := 99999999999
	indentedLinesCnt := 0

	// lineOffsets tells you where the beginning of each line is in terms of
	// offset. Example:
	//  "\tfoo\n\tbar\n"       ->   [0, 5]
	//   0       5
	var lineOffsets []int

	// For code readability purposes, users can leave the first line empty.
	if s[0] != '\n' {
		lineOffsets = append(lineOffsets, 0)
	}

	curLineIndent := 0 // Number of tabs or spaces in the current line.
	for pos := range s {
		if s[pos] == '\n' {
			if pos+1 < len(s) {
				lineOffsets = append(lineOffsets, pos+1)
			}
			curLineIndent = 0
			continue
		}

		// Skip to the next line if we are already beyond the minimal indent
		// level that we have found so far. The rest of this line will be kept
		// as-is.
		if curLineIndent >= indentsPerLine {
			continue
		}

		// The minimal indent level that we have found so far in previous lines
		// might not be the smallest indent level. Once we hit the first
		// non-indent char, let's check whether it is the new minimal indent
		// level.
		if s[pos] != ' ' && s[pos] != '\t' {
			if curLineIndent != 0 {
				indentedLinesCnt++
			}
			indentsPerLine = curLineIndent
			continue
		}

		curLineIndent++
	}

	// Extract each line without indentation.
	out := make([]byte, 0, len(s)-(indentsPerLine*indentedLinesCnt))

	for line := range lineOffsets {
		first := lineOffsets[line]

		// Index of the last character of the line. It is often the '\n'
		// character, except for the last line.
		var last int
		if line == len(lineOffsets)-1 {
			last = len(s) - 1
		} else {
			last = lineOffsets[line+1] - 1
		}

		var lineStr string
		switch {
		// Case 0: if the first line is empty, let's skip it.
		case line == 0 && first == last:
			lineStr = ""

		// Case 1: we want the user to be able to omit some tabs or spaces in
		// the last line for readability purposes.
		case line == len(lineOffsets)-1 && s[last] != '\n' && isIndent(s[first:last+1]):
			lineStr = ""

		// Case 2: we want the user to be able to omit the indentations for
		// empty lines for readability purposes.
		case first == last:
			lineStr = "\n"

		// Case 3: error when a line doesn't contain the correct indentation
		// level.
		case first+indentsPerLine > last:
			panic(fmt.Sprintf("line %d has an incorrect indent level: %q", line, s[first:last]))

		// Case 4: at this point, the indent level is correct, so let's remove
		// the indentation and keep the rest.
		case first+indentsPerLine <= last:
			lineStr = s[first+indentsPerLine : last+1]

		default:
			panic(fmt.Sprintf("unexpected case: first: %d, last: %d, indentsPerLine: %d, line: %q", first, last, indentsPerLine, s[first:last]))
		}
		out = append(out, lineStr...)
	}

	return string(out)
}

// isIndent returns true if the given string is only made of spaces or a
// tabs.
func isIndent(s string) bool {
	for _, r := range s {
		if r != ' ' && r != '\t' {
			return false
		}
	}
	return true
}
