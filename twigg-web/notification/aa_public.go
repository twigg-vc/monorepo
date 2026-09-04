package notification

type Notification struct {
	Id        int64
	UserId    int64
	Message   string
	AssetPath string
	CreatedAt string
	SeenAt    string
	ReadAt    string
}
