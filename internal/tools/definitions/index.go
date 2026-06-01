package definitions

import (
	"os"

	"github.com/CaowardlyLion/OpenHome/internal/tools"
)

func All() []tools.Definition {
	return []tools.Definition{
		ListFiles(), ReadFile(), SearchFiles(), WriteFile(),
		ReadExternalFile(), RunCommand(), FetchURL(), DownloadFile(), WebSearch(DuckDuckGoHTML{Endpoint: os.Getenv("OPENHOME_SEARCH_ENDPOINT")}),
	}
}
