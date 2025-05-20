package cmd

import "github.com/Hayao0819/Kamisato/internal/utils"

func init(){
	subCmds.Add(utils.TodoCmd("diff-build"))
}
