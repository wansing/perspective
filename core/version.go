package core

type DBVersionStub interface {
	TsChanged() int64
	VersionNo() int // ascending
	VersionNote() string
	WorkflowGroupId() int // one from the workflow, or zero if the version finished its workflow and is visible to all who have the right to read it
}

type DBVersion interface {
	DBVersionStub
	Content() string
}

// VersionWrapContent implements DBVersion and overrides Content().
type VersionWrapContent struct {
	DBVersion
	content string
}

func (v VersionWrapContent) Content() string {
	return v.content
}
