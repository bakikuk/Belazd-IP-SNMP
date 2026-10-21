package capture

type multiRunner struct {
	parts []Runner
}

func (m *multiRunner) Start() error {
	for _, p := range m.parts {
		if err := p.Start(); err != nil {
			m.Stop()
			return err
		}
	}
	return nil
}

func (m *multiRunner) Stop() {
	for _, p := range m.parts {
		if p != nil {
			p.Stop()
		}
	}
}
