package notify

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/user"
	"strings"
)

// Notify interface.
type Notify interface {
	Send(ctx context.Context, message string)
}

// New returns Notice.
func New(ctx context.Context, url string) (Notify, error) {
	splitted := strings.SplitN(url, "://", 2)

	switch splitted[0] {
	case "":
		return &Null{}, nil
	case "slack":
		sl, err := NewSlack(splitted[1])
		if err != nil {
			log.Printf("[ERROR] %s", err)
			return &Null{}, nil
		}
		return sl, nil
	case "mail", "smtp":
		ml, err := NewMail(splitted[1])
		if err != nil {
			log.Printf("[ERROR] %s", err)
			return &Null{}, nil
		}
		return ml, nil
	default:
		return nil, fmt.Errorf("unsupported notify: %s", url)
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
