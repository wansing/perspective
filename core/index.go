package core

// An IndexDB stores timestamps and tags which are defined in node content.
type IndexDB interface {
	AddTags(nodeId int, versionTsChanged int64, tags []string) error
	AddTimestamps(nodeId int, timestamps []int64) error
	ClearIndex(nodeId int) error
	RecentByTag(now int64, tag string, limit, offset int) ([]int, error)   // uses tsChanged of max released version
	UpcomingByTag(now int64, tag string, limit, offset int) ([]int, error) // uses timestamp
}
