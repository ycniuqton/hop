package cli

// registeredCommands lists every v1 command in help order.
func registeredCommands() []Command {
	return []Command{
		&InitCommand{}, &InviteCommand{}, &JoinCommand{}, &DevicesCommand{}, &WhoamiCommand{},
		&RenameCommand{}, &RemoveCommand{}, &RotateCommand{}, &StatusCommand{}, &CompletionCommand{},
		&PushCommand{}, &PullCommand{}, &HotCommand{}, &KeepCommand{},
		&CopyCommand{}, &ListCommand{}, &MoveCommand{}, &RemoveStoreCommand{},
		&RefreshCommand{},
	}
}
