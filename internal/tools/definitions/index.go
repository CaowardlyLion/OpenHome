package definitions

import "github.com/CaowardlyLion/OpenHome/internal/tools"

func All() []tools.Definition {
	return []tools.Definition{ListFiles(), ReadFile(), SearchFiles(), WriteFile()}
}
