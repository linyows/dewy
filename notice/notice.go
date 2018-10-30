package notice

import "context"

type Notice interface {
	String() string
	Notify(ctx context.Context, message string)
}

type Field struct {
	Title string
	Value string
	Short bool
}

type NoticeConfig struct {
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

func New(n Notice) Notice {
	switch n.String() {
	case "slack":
		return n
	default:
		panic("no noticer")
	}
}
