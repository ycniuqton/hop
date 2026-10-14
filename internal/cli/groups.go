package cli

// CommandGroup is the help-section a command is listed under.
type CommandGroup string

const (
	GroupSetup CommandGroup = "setup"
	GroupHot   CommandGroup = "hot"
	GroupStore CommandGroup = "store"
)

var groupTitles = map[CommandGroup]string{
	GroupSetup: "Setup and devices:",
	GroupHot:   "Hot shelves (names only):",
	GroupStore: "Store (paths):",
}

// Title is the heading shown in `hop --help`.
func (g CommandGroup) Title() string { return groupTitles[g] }

// CommandGroups lists groups in help order.
func CommandGroups() []CommandGroup { return []CommandGroup{GroupSetup, GroupHot, GroupStore} }
