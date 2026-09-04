package cicdqueue

type service struct {
	js            JobsStorage
	db            Db
	ciCdPublisher CiCdJobsPoster
	queue         Queue
}

func newService(js JobsStorage, ciCdPublisher CiCdJobsPoster,
	db Db, queue Queue) (Service, error) {
	s := service{
		js:            js,
		db:            db,
		ciCdPublisher: ciCdPublisher,
		queue:         queue,
	}
	onStartAutoCiCdRunDeadLetter := func(p []byte) error {
		return nil
	}
	queue.Register(QueueStartAutoCiCdRunPayloadType,
		s.startAutoCiCdRun,
		s.startCiCdRunPayloadDisplayString, onStartAutoCiCdRunDeadLetter)
	onResumeCdDeadLetter := func(p []byte) error {
		return nil
	}
	queue.Register(QueueResumeCdPayloadType, s.resumeCd,
		s.resumeCdPayloadDisplayString, onResumeCdDeadLetter)
	return Service{s}, nil
}
