package bugsearch

func parseQuery(repoId uint64, q string) (f Filter, err error) {
	return Filter{RepoId: repoId}, nil
}
