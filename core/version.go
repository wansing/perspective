package core

import "github.com/wansing/perspective/util"

type DBVersionStub interface {
	TsChanged() int64
	VersionNo() int // ascending
	VersionNote() string
	WorkflowGroupID() int // one from the workflow, or zero if the version finished its workflow and is visible to all who have the right to read it
}

type DBVersion interface {
	DBVersionStub
	Content() string
}

type Version struct {
	DBVersion
	NewContent    string
	HasNewContent bool
	Tags          []string
	Timestamps    []int64
}

func NewVersion(dbVersion DBVersion) *Version {
	return &Version{
		DBVersion:  dbVersion,
		Tags:       []string{},
		Timestamps: []int64{},
	}
}

func (v *Version) Content() string {
	if v == nil {
		return ""
	}
	if v.HasNewContent {
		return v.NewContent
	} else {
		return v.DBVersion.Content()
	}
}

func (v *Version) SetContent(content string) {
	if v == nil {
		return
	}
	v.NewContent = content
	v.HasNewContent = true
}

// Tags adds one or more tags to the current version.
//
// For usage in templates, funcs must return "one return value (of any type) or two return values, the second of which is an error."
func (v *Version) Tag(tags ...string) interface{} {
	v.Tags = append(v.Tags, tags...)
	return nil
}

// Ts adds one or more timestamps to the current version. Arguments are parsed with util.ParseTime.
//
// For usage in templates, funcs must return "one return value (of any type) or two return values, the second of which is an error."
func (v *Version) Ts(dates ...string) interface{} {
	for _, dateStr := range dates {
		if ts, err := util.ParseTime(dateStr); err == nil {
			v.Timestamps = append(v.Timestamps, ts)
		}
	}
	return nil
}
