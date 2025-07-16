package dewy

import (
	"fmt"
	"io"
)

// Banner displays the Dewy ASCII art logo
func Banner(w io.Writer) {
	fmt.Fprint(w, `
 ___   ___  _____ __  __
|   \ | __| \  / /\ \/ /
|   | | __| \ / /  \  /
|___/ |___| \_/_/  |__|

`)
}
