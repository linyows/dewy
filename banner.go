package dewy

import (
	"fmt"
	"io"
)

// Banner displays the Dewy ASCII art logo
func Banner(w io.Writer) {
	fmt.Fprint(w, `
█▀▄ █▀▀ █ █ █ █
█ █ █▀▀ █▄█ ▀█▀
▀▀  ▀▀▀  ▀   ▀ 
`)
}
