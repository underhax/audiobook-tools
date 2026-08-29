package cli

import (
	"flag"
	"fmt"
	"strings"
)

func setupUsage(fs *flag.FlagSet, name string) {
	fs.Usage = func() {
		out := fs.Output()
		if _, err := fmt.Fprintf(out, "Usage of %s:\n", name); err != nil {
			return
		}
		fs.VisitAll(func(f *flag.Flag) {
			prefix := "--"
			if len(f.Name) <= 1 {
				prefix = "-"
			}
			s := fmt.Sprintf("  %s%s", prefix, f.Name)
			typeName, usage := flag.UnquoteUsage(f)
			if typeName != "" {
				s += " " + typeName
			}
			if len(s) <= 4 {
				s += "\t"
			} else {
				s += "\n    \t"
			}
			s += strings.ReplaceAll(usage, "\n", "\n    \t")
			if f.DefValue != "" && f.DefValue != "false" && f.DefValue != "0" && f.DefValue != `""` {
				s += fmt.Sprintf(" (default %s)", f.DefValue)
			}
			if _, err := fmt.Fprintln(out, s); err != nil {
				return
			}
		})
	}
}
