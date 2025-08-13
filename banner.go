package dewy

import (
	"io"
	"strings"

	"github.com/fatih/color"
)

// Banner displays the Dewy ASCII art logo
func Banner(w io.Writer) {
	green := color.RGB(194, 73, 85)
	grey := color.New(color.FgHiBlack)

	green.Fprint(w, strings.TrimLeft(`
 ___   ___  _____ __  __
|   \ | __| \  / /\ \/ /
|   | | __| \ / /  \  /
|___/ |___| \_/_/  |__|
`, "\n"))
	grey.Fprint(w, `
Dewy - A declarative deployment tool of apps in non-K8s environments.
https://github.com/linyows/dewy

`)
}
