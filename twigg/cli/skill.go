package cli

import _ "embed"

//go:embed SKILL.md
var skillMarkdown string

func (a *app) skill(args commandArgs) {
	a.out.Write([]byte(skillMarkdown))
}
