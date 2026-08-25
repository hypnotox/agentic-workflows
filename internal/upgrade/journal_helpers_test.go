package upgrade

import "github.com/hypnotox/agentic-workflows/internal/manifest"

func imageOf(root, path string) (Image, error) {
	return productionJournalOperation().imageOf(root, path)
}

func applyImage(root, path string, image Image) error {
	return productionJournalOperation().applyImage(root, path, image)
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

func cutoverOperations(root string, lock *manifest.Lock) ([]Operation, error) {
	return cutoverOperationsWith(root, lock, productionJournalOperation())
}
