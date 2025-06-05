package cmd

import utils "github.com/Hayao0819/Kamisato/internal"

func init() {
	subCmds.Add(utils.TodoCmd("diff-upload"))
}
