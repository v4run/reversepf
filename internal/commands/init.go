package commands

func NewInitCommand() Command {
	return Command{
		Type: TypeInit,
	}
}
