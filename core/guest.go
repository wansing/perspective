package core

type Guest struct{}

func (Guest) ID() int {
	return 0
}

func (Guest) Name() string {
	return "Guest"
}
