package testutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// This is a test for the testing func "Undent". I wasn't confident with
// Undent's behavior, so I wrote this test to verify it.
func Test_Undent(t *testing.T) {
	t.Run("empty string", runTest_Undent(``, ``))

	t.Run("if last line has the same indent as other lines and, it is ignored", runTest_Undent(`
		foo
		bar
		`, "foo\nbar\n"))

	t.Run("you can un-indent the last line to make the Go code more readable", runTest_Undent(`
		foo
		bar
	`, "foo\nbar\n"))

	t.Run("last line may not be an empty line", runTest_Undent(`
		foo
		bar`, "foo\nbar"))

	t.Run("1 empty line is preserved", runTest_Undent("\t\tfoo\n\t\t\n\t\tbar\n", "foo\n\nbar\n"))

	t.Run("2 empty lines are preserved", runTest_Undent("\t\tfoo\n\t\t\n\t\t\n\t\tbar\n", "foo\n\n\nbar\n"))

	t.Run("you can also omit the tabs or spaces for empty lines", runTest_Undent(`
		foo

		bar
	`, "foo\n\nbar\n"))
}

func runTest_Undent(given, expected string) func(t *testing.T) {
	return func(t *testing.T) {
		t.Helper()
		got := Undent(given)
		assert.Equal(t, expected, got)
	}
}
