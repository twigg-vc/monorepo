package repo

type repo struct {
	quotaOwner string
	id         uint64
}

func newRepo(quotaOwner string, id uint64) repo {
	return repo{quotaOwner, id}
}