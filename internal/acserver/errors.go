package acserver

func errorGroup(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}

	return nil
}

type groupedError []error

func (e groupedError) Err() error {
	for _, err := range e {
		if err != nil {
			return err
		}
	}

	return nil
}
