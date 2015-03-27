package smtp

type heloCmd struct {
	domain string
	isEhlo bool
}

type mailFromCmd struct {
	from string
}

type rcptToCmd struct {
	to string
}

type dataCmd struct {
}

type rsetCmd struct {
}

type noopCmd struct {
}

type quitCmd struct {
}

type vrfyCmd struct {
}

type bdatCmd struct {
	length int
	last   bool
}
