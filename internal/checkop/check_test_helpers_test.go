package checkop

type failOnWrite struct {
	failAt int
	err    error
	writes int
}

func (w *failOnWrite) Write(p []byte) (int, error) {
	w.writes++
	if w.writes >= w.failAt {
		return 0, w.err
	}
	return len(p), nil
}
