package notice

import "context"

// Notice interface
type Notice interface {
	String() string
	Notify(ctx context.Context, message string)
}

// Field struct
type Field struct {
	Title string
	Value string
	Short bool
}

// Config struct
type Config struct {
	Host             string
	Command          string
	User             string
	Source           string
	WorkingDirectory string
	RepoOwner        string
	RepoName         string
	RepoLink         string
	RepoOwnerIcon    string
	RepoOwnerLink    string
}

// New returns Notice
func New(n Notice) Notice {
	switch n.String() {
	case "slack":
		return n
	default:
		panic("no noticer")
	}
}
