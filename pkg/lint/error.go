package lint

type LintError struct {
	PackagePath string
	Lint        string
	Err         error
}
