package notice

import (
	"context"
	"fmt"
	"os"
	"os/user"
)

// Notice interface.
type Notice interface {
	String() string
	Notify(ctx context.Context, message string)
}

// Field struct.
type Field struct {
	Title string
	Value string
	Short bool
}

// Config struct.
type Config struct {
	Command   string
	Source    string
	Owner     string
	Repo      string
	RepoLink  string
	OwnerIcon string
	OwnerLink string
}

// New returns Notice.
func New(n Notice) (Notice, error) {
	switch n.String() {
	case "slack":
		return n, nil
	default:
		return nil, fmt.Errorf("no noticer")
	}
}

func hostname() string {
	n, err := os.Hostname()
	if err != nil {
		return fmt.Sprintf("%#v", err)
	}
	return n
}

func cwd() string {
	c, err := os.Getwd()
	if err != nil {
		return fmt.Sprintf("%#v", err)
	}
	return c
}

func username() string {
	u, err := user.Current()
	if err != nil {
		return fmt.Sprintf("%#v", err)
	}
	return u.Name
}
