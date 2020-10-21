package core

type Guest struct{}

func (Guest) Id() int {
	return 0
}

func (Guest) Name() string {
	return "Guest"
}
