package git

type GitInterface interface {
	GitWithEditor(args string, output *string, editorCmd string) error
	Git(args string, output *string) error
	RootDir() string
}
