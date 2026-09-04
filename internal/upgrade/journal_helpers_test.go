package upgrade

func imageOf(root, path string) (Image, error) {
	return productionJournalOperation().imageOf(root, path)
}

func commitTransaction(root string, operations []Operation) (Outcome, error) {
	return commitTransactionWith(root, operations, productionJournalOperation())
}

func commitTransactionWith(root string, operations []Operation, operation journalOperation) (outcome Outcome, err error) {
	err = withBoundJournalOperation(root, operation, func(bound journalOperation) error {
		var runErr error
		outcome, runErr = commitTransactionBound(root, operations, bound)
		return runErr
	})
	return outcome, err
}
