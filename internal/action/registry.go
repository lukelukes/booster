package action

import (
	"booster/internal/expr"

	"gopkg.in/yaml.v3"
)

type Factory func(args yaml.Node, ctx *expr.Context) (Action, error)

func Registry() map[string]Factory {
	return map[string]Factory{
		"dir.create":     NewDirCreate,
		"symlink.create": NewSymlinkCreate,
	}
}
